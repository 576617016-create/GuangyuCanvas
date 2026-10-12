package outbound

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// allowLoopback pins the exact test host so the SSRF policy admits httptest.
func allowLoopback(t *testing.T) {
	t.Helper()
	t.Setenv("CANVAS_ALLOW_PRIVATE_UPSTREAMS", "false")
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
}

func resumableTestPayload(size int) []byte {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	return payload
}

func parseTestRange(header string, total int64) (int64, bool) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, false
	}
	spec := strings.TrimPrefix(header, "bytes=")
	dash := strings.Index(spec, "-")
	if dash < 0 {
		return 0, false
	}
	start, err := strconv.ParseInt(strings.TrimSpace(spec[:dash]), 10, 64)
	if err != nil || start < 0 || start >= total {
		return 0, false
	}
	return start, true
}

func writeTestArtifact(w http.ResponseWriter, payload []byte, start int64) {
	body := payload[start:]
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("ETag", `"artifact-v1"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if start > 0 {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_, _ = w.Write(body)
}

func cutTestConnection(w http.ResponseWriter, body []byte, ratio float64) {
	cut := int(float64(len(body)) * ratio)
	_, _ = w.Write(body[:cut])
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	panic(http.ErrAbortHandler)
}

func TestDownloadResumableResumesAfterConnectionCut(t *testing.T) {
	allowLoopback(t)
	payload := resumableTestPayload(30000)
	var mu sync.Mutex
	requests := 0
	rangeRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		attempt := requests
		if r.Header.Get("Range") != "" {
			rangeRequests++
		}
		mu.Unlock()
		start := int64(0)
		if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
			parsed, ok := parseTestRange(rangeHeader, int64(len(payload)))
			if !ok {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			start = parsed
		}
		if attempt == 1 {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)-int(start)))
			w.WriteHeader(http.StatusOK)
			cutTestConnection(w, payload[start:], 0.4)
			return
		}
		writeTestArtifact(w, payload, start)
	}))
	defer server.Close()

	partial := filepath.Join(t.TempDir(), "artifact.part")
	result, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 1 << 20, Attempts: 4})
	if err != nil {
		t.Fatalf("DownloadResumable() error = %v", err)
	}
	if !bytes.Equal(result.Data, payload) {
		t.Fatalf("resumed bytes differ: got %d want %d", len(result.Data), len(payload))
	}
	if rangeRequests != 1 {
		t.Fatalf("range requests = %d, want 1", rangeRequests)
	}
	if result.MimeType != "image/png" || result.FileName != "artifact.png" {
		t.Fatalf("result meta = %q %q", result.MimeType, result.FileName)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatalf("partial file was not removed")
	}
	if _, err := os.Stat(partial + ".json"); !os.IsNotExist(err) {
		t.Fatalf("sidecar was not removed")
	}
}

func TestDownloadResumableRestartsWhenServerIgnoresRange(t *testing.T) {
	allowLoopback(t)
	payload := resumableTestPayload(24000)
	var mu sync.Mutex
	requests := 0
	rangeRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		attempt := requests
		if r.Header.Get("Range") != "" {
			rangeRequests++
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Accept-Ranges", "none")
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		if attempt == 1 {
			cutTestConnection(w, payload, 0.4)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	partial := filepath.Join(t.TempDir(), "artifact.part")
	result, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 1 << 20, Attempts: 4})
	if err != nil {
		t.Fatalf("DownloadResumable() error = %v", err)
	}
	if !bytes.Equal(result.Data, payload) {
		t.Fatalf("restart bytes differ: got %d want %d", len(result.Data), len(payload))
	}
	if rangeRequests != 1 {
		t.Fatalf("range requests = %d, want 1", rangeRequests)
	}
	if requests < 2 {
		t.Fatalf("requests = %d, want at least 2", requests)
	}
}

func TestDownloadResumableRestartsWhenValidatorChanges(t *testing.T) {
	allowLoopback(t)
	payload := resumableTestPayload(20000)
	var mu sync.Mutex
	requests := 0
	rangeAccepted := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		attempt := requests
		mu.Unlock()
		start := int64(0)
		if r.Header.Get("Range") != "" && r.Header.Get("If-Range") == `"artifact-v2"` {
			mu.Lock()
			rangeAccepted++
			mu.Unlock()
			parsed, ok := parseTestRange(r.Header.Get("Range"), int64(len(payload)))
			if ok {
				start = parsed
			}
		}
		body := payload[start:]
		w.Header().Set("Content-Type", "image/png")
		// The first response advertises a stale validator that the client stores.
		if attempt == 1 {
			w.Header().Set("ETag", `"artifact-v1"`)
		} else {
			w.Header().Set("ETag", `"artifact-v2"`)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		if start > 0 {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		if attempt == 1 {
			cutTestConnection(w, body, 0.4)
			return
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()

	partial := filepath.Join(t.TempDir(), "artifact.part")
	result, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 1 << 20, Attempts: 4})
	if err != nil {
		t.Fatalf("DownloadResumable() error = %v", err)
	}
	if !bytes.Equal(result.Data, payload) {
		t.Fatalf("validator-change bytes differ: got %d want %d", len(result.Data), len(payload))
	}
	if rangeAccepted != 0 {
		t.Fatalf("stale If-Range was accepted %d times, want 0", rangeAccepted)
	}
	if requests < 2 {
		t.Fatalf("requests = %d, want at least 2", requests)
	}
}

func TestDownloadResumableIdleTimeoutResumesAndSteadyTransferSurvives(t *testing.T) {
	allowLoopback(t)

	t.Run("stall triggers retry that resumes", func(t *testing.T) {
		payload := resumableTestPayload(20000)
		var mu sync.Mutex
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requests++
			attempt := requests
			mu.Unlock()
			start := int64(0)
			if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
				parsed, ok := parseTestRange(rangeHeader, int64(len(payload)))
				if !ok {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				start = parsed
			}
			body := payload[start:]
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			if start > 0 {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
				w.WriteHeader(http.StatusPartialContent)
			} else {
				w.WriteHeader(http.StatusOK)
			}
			_, _ = w.Write(body[:5000])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			if attempt == 1 {
				time.Sleep(500 * time.Millisecond)
			}
			_, _ = w.Write(body[5000:])
		}))
		defer server.Close()

		partial := filepath.Join(t.TempDir(), "artifact.part")
		result, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 1 << 20, Attempts: 4, IdleTimeout: 150 * time.Millisecond})
		if err != nil {
			t.Fatalf("DownloadResumable() error = %v", err)
		}
		if !bytes.Equal(result.Data, payload) {
			t.Fatalf("idle-resume bytes differ: got %d want %d", len(result.Data), len(payload))
		}
		if requests < 2 {
			t.Fatalf("requests = %d, want at least 2", requests)
		}
	})

	t.Run("slow but steady transfer is not an idle failure", func(t *testing.T) {
		payload := resumableTestPayload(20000)
		var mu sync.Mutex
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requests++
			mu.Unlock()
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)
			chunk := 1000
			for offset := 0; offset < len(payload); offset += chunk {
				end := offset + chunk
				if end > len(payload) {
					end = len(payload)
				}
				_, _ = w.Write(payload[offset:end])
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				time.Sleep(40 * time.Millisecond)
			}
		}))
		defer server.Close()

		partial := filepath.Join(t.TempDir(), "artifact.part")
		result, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 1 << 20, Attempts: 4, IdleTimeout: 150 * time.Millisecond})
		if err != nil {
			t.Fatalf("DownloadResumable() error = %v", err)
		}
		if !bytes.Equal(result.Data, payload) {
			t.Fatalf("steady bytes differ: got %d want %d", len(result.Data), len(payload))
		}
		if requests != 1 {
			t.Fatalf("steady transfer retried: requests = %d, want 1", requests)
		}
	})
}

func TestDownloadResumableRejectsOversizeAndDeletesPartial(t *testing.T) {
	allowLoopback(t)
	payload := resumableTestPayload(10000)

	t.Run("declared content length", func(t *testing.T) {
		var mu sync.Mutex
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requests++
			mu.Unlock()
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		}))
		defer server.Close()

		partial := filepath.Join(t.TempDir(), "artifact.part")
		_, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 5000, Attempts: 4})
		if !errors.Is(err, ErrResourceTooLarge) {
			t.Fatalf("error = %v, want ErrResourceTooLarge", err)
		}
		if requests != 1 {
			t.Fatalf("requests = %d, want 1 (non-retryable)", requests)
		}
		if _, err := os.Stat(partial); !os.IsNotExist(err) {
			t.Fatalf("partial file survived oversize rejection")
		}
		if _, err := os.Stat(partial + ".json"); !os.IsNotExist(err) {
			t.Fatalf("sidecar survived oversize rejection")
		}
	})

	t.Run("unknown content length", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			for offset := 0; offset < len(payload); offset += 1000 {
				end := offset + 1000
				if end > len(payload) {
					end = len(payload)
				}
				_, _ = w.Write(payload[offset:end])
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		}))
		defer server.Close()

		partial := filepath.Join(t.TempDir(), "artifact.part")
		_, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 5000, Attempts: 4})
		if !errors.Is(err, ErrResourceTooLarge) {
			t.Fatalf("error = %v, want ErrResourceTooLarge", err)
		}
		if _, err := os.Stat(partial); !os.IsNotExist(err) {
			t.Fatalf("partial file survived streaming oversize")
		}
	})
}

func TestDownloadResumableNotFoundIsNotRetried(t *testing.T) {
	allowLoopback(t)
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	partial := filepath.Join(t.TempDir(), "artifact.part")
	_, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 1 << 20, Attempts: 4})
	if err == nil {
		t.Fatal("DownloadResumable() should fail on 404")
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1 (404 is non-retryable)", requests)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatalf("partial survived 404")
	}
	if _, err := os.Stat(partial + ".json"); !os.IsNotExist(err) {
		t.Fatalf("sidecar survived 404")
	}
}

func TestDownloadResumablePartialPersistsAcrossCalls(t *testing.T) {
	allowLoopback(t)
	payload := resumableTestPayload(18000)
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		attempt := requests
		mu.Unlock()
		start := int64(0)
		if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
			parsed, ok := parseTestRange(rangeHeader, int64(len(payload)))
			if !ok {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			start = parsed
		}
		body := payload[start:]
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"artifact-v1"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		if start > 0 {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		if attempt == 1 {
			cutTestConnection(w, body, 0.4)
			return
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()

	partial := filepath.Join(t.TempDir(), "artifact.part")
	if _, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 1 << 20, Attempts: 1}); err == nil {
		t.Fatal("first call with a single attempt should fail on the cut")
	}
	info, err := os.Stat(partial)
	if err != nil || info.Size() == 0 {
		t.Fatalf("partial did not survive the failed call: %v", err)
	}
	if _, err := os.Stat(partial + ".json"); err != nil {
		t.Fatalf("sidecar did not survive the failed call: %v", err)
	}

	result, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 1 << 20, Attempts: 4})
	if err != nil {
		t.Fatalf("second call error = %v", err)
	}
	if !bytes.Equal(result.Data, payload) {
		t.Fatalf("cross-call resume bytes differ: got %d want %d", len(result.Data), len(payload))
	}
}

func TestDownloadResumableSerializesSamePartialPath(t *testing.T) {
	allowLoopback(t)
	payload := resumableTestPayload(16000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestArtifact(w, payload, 0)
	}))
	defer server.Close()

	partial := filepath.Join(t.TempDir(), "artifact.part")
	var wg sync.WaitGroup
	results := make([][]byte, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result, err := DownloadResumable(context.Background(), server.URL+"/artifact.png", ResumableOptions{PartialPath: partial, MaxBytes: 1 << 20, Attempts: 2})
			results[index], errs[index] = result.Data, err
		}(i)
	}
	wg.Wait()
	for index := range results {
		if errs[index] != nil {
			t.Fatalf("concurrent download %d error = %v", index, errs[index])
		}
		if !bytes.Equal(results[index], payload) {
			t.Fatalf("concurrent download %d bytes differ: got %d want %d", index, len(results[index]), len(payload))
		}
	}
}
