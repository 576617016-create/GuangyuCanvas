package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrResourceTooLarge marks a download that exceeded the caller's hard byte cap.
// It is non-retryable so the caller can map it to the product size-limit copy.
var ErrResourceTooLarge = errors.New("远程资源超过大小上限")

// ResumableOptions configures DownloadResumable. There is deliberately no total
// timeout: a slow but steady generation artifact must be able to finish.
type ResumableOptions struct {
	PartialPath   string
	MaxBytes      int64
	HeaderTimeout time.Duration
	IdleTimeout   time.Duration
	Attempts      int
}

// ResumableResult is a fully materialized artifact. Data is read from the .part
// file only after the transfer completed, never accumulated during transfer.
type ResumableResult struct {
	Data     []byte
	MimeType string
	FileName string
}

const (
	defaultResumableAttempts      = 4
	defaultResumableHeaderTimeout = 30 * time.Second
	defaultResumableIdleTimeout   = 45 * time.Second
	resumableReadBuffer           = 32 << 10
)

type resumableSidecar struct {
	URL          string `json:"url"`
	ETag         string `json:"etag"`
	LastModified string `json:"lastModified"`
	Total        int64  `json:"total"`
}

// DownloadResumable streams a remote artifact to PartialPath and resumes from
// the existing bytes across transient failures. A sidecar file
// (PartialPath+".json") records the source URL, validator and total size so a
// later attempt can send Range/If-Range. Concurrent downloads of the same
// PartialPath are serialized in-process.
func DownloadResumable(ctx context.Context, rawURL string, opts ResumableOptions) (ResumableResult, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ResumableResult{}, fmt.Errorf("外部资源地址无效")
	}
	if strings.TrimSpace(opts.PartialPath) == "" {
		return ResumableResult{}, errors.New("下载缓存路径未配置")
	}
	if opts.MaxBytes <= 0 {
		return ResumableResult{}, errors.New("下载大小上限未配置")
	}
	attempts := opts.Attempts
	if attempts <= 0 {
		attempts = defaultResumableAttempts
	}
	headerTimeout := opts.HeaderTimeout
	if headerTimeout <= 0 {
		headerTimeout = defaultResumableHeaderTimeout
	}
	idleTimeout := opts.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = defaultResumableIdleTimeout
	}
	if err := os.MkdirAll(filepath.Dir(opts.PartialPath), 0o700); err != nil {
		return ResumableResult{}, err
	}

	unlock := lockResumableDownload(opts.PartialPath)
	defer unlock()

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := waitResumableBackoff(ctx, attempt); err != nil {
				return ResumableResult{}, err
			}
		}
		result, retryable, err := downloadResumableAttempt(ctx, parsed, opts, headerTimeout, idleTimeout)
		if err == nil {
			return result, nil
		}
		if ctx.Err() != nil {
			return ResumableResult{}, ctx.Err()
		}
		if !retryable {
			return ResumableResult{}, err
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("远程资源下载失败")
	}
	return ResumableResult{}, lastErr
}

func downloadResumableAttempt(ctx context.Context, parsed *url.URL, opts ResumableOptions, headerTimeout, idleTimeout time.Duration) (ResumableResult, bool, error) {
	sidecar := loadResumableSidecar(opts.PartialPath)
	offset := int64(0)
	if sidecar != nil && sidecar.URL == parsed.String() {
		if info, err := os.Stat(opts.PartialPath); err == nil && info.Size() > 0 {
			offset = info.Size()
		}
	}
	if offset == 0 {
		removeResumablePartial(opts.PartialPath)
		sidecar = nil
	}

	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	headerTimer := time.AfterFunc(headerTimeout, cancel)

	request, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		headerTimer.Stop()
		return ResumableResult{}, false, err
	}
	ApplyDefaultOutboundHeaders(request)
	if offset > 0 && sidecar != nil {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		if strings.TrimSpace(sidecar.ETag) != "" {
			request.Header.Set("If-Range", sidecar.ETag)
		} else if strings.TrimSpace(sidecar.LastModified) != "" {
			request.Header.Set("If-Range", sidecar.LastModified)
		}
	}
	response, err := OutboundHTTPClient(0).Do(request)
	headerTimer.Stop()
	if err != nil {
		return ResumableResult{}, true, err
	}
	defer response.Body.Close()

	total := int64(-1)
	pending := resumableSidecar{
		URL:          parsed.String(),
		ETag:         response.Header.Get("ETag"),
		LastModified: response.Header.Get("Last-Modified"),
	}
	switch response.StatusCode {
	case http.StatusRequestedRangeNotSatisfiable:
		removeResumablePartial(opts.PartialPath)
		return ResumableResult{}, true, fmt.Errorf("远程资源范围请求无效：%s", response.Status)
	case http.StatusOK:
		// The server ignored Range, or the validator changed. Restart from zero.
		offset = 0
		if declared := response.ContentLength; declared >= 0 {
			total = declared
		}
	case http.StatusPartialContent:
		start, contentTotal, ok := parseContentRange(response.Header.Get("Content-Range"))
		if !ok || start != offset {
			removeResumablePartial(opts.PartialPath)
			return ResumableResult{}, true, errors.New("远程资源断点位置不匹配")
		}
		// A known total that no longer matches means the remote object changed
		// size under the same validator; appending would corrupt the artifact.
		if sidecar != nil && sidecar.Total >= 0 && contentTotal >= 0 && sidecar.Total != contentTotal {
			removeResumablePartial(opts.PartialPath)
			return ResumableResult{}, true, errors.New("远程资源总大小已变化")
		}
		total = contentTotal
	default:
		if retryableHTTPStatus(response.StatusCode) {
			return ResumableResult{}, true, fmt.Errorf("远程资源下载失败：%s", response.Status)
		}
		removeResumablePartial(opts.PartialPath)
		return ResumableResult{}, false, fmt.Errorf("远程资源下载失败：%s", response.Status)
	}
	pending.Total = total
	if total >= 0 && total >= opts.MaxBytes {
		removeResumablePartial(opts.PartialPath)
		return ResumableResult{}, false, ErrResourceTooLarge
	}
	// Persist the sidecar before streaming. A process crash or kill mid-transfer
	// never runs a failure path, and without the sidecar the next call would
	// discard the partial and restart from byte zero.
	saveResumableSidecar(opts.PartialPath, pending)

	idleTimer := time.AfterFunc(idleTimeout, cancel)
	defer idleTimer.Stop()
	reader := &idleResetReader{body: response.Body, timer: idleTimer, idle: idleTimeout}

	file, err := os.OpenFile(opts.PartialPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return ResumableResult{}, false, err
	}
	if offset == 0 {
		if err := file.Truncate(0); err != nil {
			_ = file.Close()
			return ResumableResult{}, false, err
		}
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		_ = file.Close()
		return ResumableResult{}, false, err
	}

	written := offset
	buffer := make([]byte, resumableReadBuffer)
	var readErr error
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			if written+int64(n) >= opts.MaxBytes {
				_ = file.Close()
				removeResumablePartial(opts.PartialPath)
				return ResumableResult{}, false, ErrResourceTooLarge
			}
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				_ = file.Close()
				return ResumableResult{}, true, writeErr
			}
			written += int64(n)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				readErr = err
			}
			break
		}
	}
	if closeErr := file.Close(); closeErr != nil && readErr == nil {
		readErr = closeErr
	}
	if readErr != nil {
		return ResumableResult{}, true, readErr
	}
	if total >= 0 && written != total {
		return ResumableResult{}, true, io.ErrUnexpectedEOF
	}

	data, err := os.ReadFile(opts.PartialPath)
	if err != nil {
		return ResumableResult{}, true, err
	}
	removeResumablePartial(opts.PartialPath)
	mimeType := normalizeResumableMimeType(response.Header.Get("Content-Type"))
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(data)
	}
	return ResumableResult{
		Data:     data,
		MimeType: mimeType,
		FileName: resumableFileName(parsed.Path),
	}, false, nil
}

func retryableHTTPStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || (status >= 500 && status <= 599)
}

func normalizeResumableMimeType(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, ";"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return value
}

func resumableFileName(urlPath string) string {
	name := path.Base(urlPath)
	if name == "" || name == "." || name == "/" || !strings.Contains(name, ".") {
		return ""
	}
	return name
}

func parseContentRange(value string) (int64, int64, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "bytes ") {
		return 0, 0, false
	}
	rest := strings.TrimPrefix(value, "bytes ")
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return 0, 0, false
	}
	rangePart := rest[:slash]
	totalPart := rest[slash+1:]
	dash := strings.Index(rangePart, "-")
	if dash < 0 {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(strings.TrimSpace(rangePart[:dash]), 10, 64)
	if err != nil {
		return 0, 0, false
	}
	if totalPart == "*" {
		return start, -1, true
	}
	total, err := strconv.ParseInt(strings.TrimSpace(totalPart), 10, 64)
	if err != nil {
		return start, -1, true
	}
	return start, total, true
}

func loadResumableSidecar(partialPath string) *resumableSidecar {
	raw, err := os.ReadFile(partialPath + ".json")
	if err != nil {
		return nil
	}
	var sidecar resumableSidecar
	if json.Unmarshal(raw, &sidecar) != nil {
		return nil
	}
	return &sidecar
}

func saveResumableSidecar(partialPath string, sidecar resumableSidecar) {
	raw, err := json.Marshal(sidecar)
	if err != nil {
		return
	}
	_ = os.WriteFile(partialPath+".json", raw, 0o600)
}

func removeResumablePartial(partialPath string) {
	_ = os.Remove(partialPath)
	_ = os.Remove(partialPath + ".json")
}

func waitResumableBackoff(ctx context.Context, attempt int) error {
	delay := time.Duration(1<<(attempt-1)) * time.Second
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type idleResetReader struct {
	body  io.ReadCloser
	timer *time.Timer
	idle  time.Duration
}

func (r *idleResetReader) Read(p []byte) (int, error) {
	n, err := r.body.Read(p)
	if n > 0 {
		r.timer.Reset(r.idle)
	}
	return n, err
}

type resumableDownloadLock struct {
	mu   sync.Mutex
	refs int
}

var (
	resumableDownloadLocksMu sync.Mutex
	resumableDownloadLocks   = map[string]*resumableDownloadLock{}
)

// lockResumableDownload serializes downloads that share a .part path. The
// delivery coordinator and the 5s recovery loop can overlap on the same
// identity, and two writers on one .part file would corrupt the resume offset.
func lockResumableDownload(partialPath string) func() {
	resumableDownloadLocksMu.Lock()
	entry := resumableDownloadLocks[partialPath]
	if entry == nil {
		entry = &resumableDownloadLock{}
		resumableDownloadLocks[partialPath] = entry
	}
	entry.refs++
	resumableDownloadLocksMu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		resumableDownloadLocksMu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(resumableDownloadLocks, partialPath)
		}
		resumableDownloadLocksMu.Unlock()
	}
}
