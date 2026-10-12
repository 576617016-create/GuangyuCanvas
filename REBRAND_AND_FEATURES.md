# 光宇无限画布 · 改版与功能说明

> 基于上游 `glanderness/BeefTV`（MIT）二次开发。本说明记录本次已完成的改动、新增功能，以及**需要你提供信息才能完全跑通的部分**。

---

## 一、品牌改名：BeefTV → 光宇无限画布

### 已改（用户可见层，共 98 个文件、323 处）
- README / QUICKSTART / docs / 功能清单等文档标题与正文。
- 应用显示名：`backend/cmd/desktop/wails.json` 的 `name` 改为「光宇无限画布」，作者改为「光宇无限画布 Contributors」。
- 前端 UI 文案、侧边栏、设置页、欢迎页等（`web/src` 下非测试源码）。
- 品牌图标与字标：`assets/readme/beeftv-wordmark.svg` 已重命名为 `guangyu-wordmark.svg` 并改为「光宇无限画布」字样；`assets/app-icon.svg`、`assets/beeftv-loading-preview.svg` 文案同步更新；README 引用已指向新文件名。

### 故意保留（避免破坏构建 / 破坏插件兼容）
- **代码标识符**：Go 包名 `beeftv`、命令目录 `backend/cmd/beeftv`、前端包名 `infinite-canvas` 不变（标准 rebrand 做法，类似 VS Code 内部代号 Code）。
- **输出二进制名**：`wails.json` 的 `outputfilename` 仍保留 `BeefTV`，与安装脚本（`.nsi`、构建脚本）引用一致，避免打包失败。
- **企业模型服务品牌 `BeefAPI`**：这是上游付费模型渠道标识，不属于产品名，保持不动。
- **`.beeftv-plugin` 扩展名 / 插件包内部结构**：插件系统契约，改名会破坏加载。
- **法律与历史文件**：`LICENSE`、`NOTICE`、`THIRD_PARTY_NOTICES.md`、`docs/release-evidence/` 保持原样。
- **测试文件**：保留原断言文案，避免测试预期错位。

> 如果你确实需要把二进制名、仓库目录也改成「光宇无限画布」，需要同步改 `scripts/`、`build/`、安装包（`.nsi`）里的所有引用，我可以再做一轮，但会显著增加改动面与构建风险。

---

## 二、接入云端 GPU

### 现状：项目已内置大量云端 GPU 插件
`plugin-packages/` 下已经覆盖主流 GPU 推理入口，基本开箱即用：

| 场景 | 已有插件 |
| --- | --- |
| AutoDL 云平台 ComfyUI | `autodl-comfyui` |
| vLLM 推理服务 | `vllm-chat` |
| LocalAI | `localai-chat` |
| Ollama（本地/云主机） | `ollama-chat` |
| LiteLLM 代理 | `litellm-proxy-chat` |
| AtlasCloud | `atlascloud-chat` |
| 任意 OpenAI 兼容端点 | `openai-chat-completions` / `compatible` 预设 |

### 本次新增：统一的「云端 GPU」入口卡片 + 仙宫云专用预设
在 `web/src/lib/model-service-presets.ts` 增加两个预设：

```ts
{ id: "cloud-gpu", name: "云端 GPU", subtitle: "AutoDL / RunPod / vLLM / LM Studio 等 OpenAI 兼容端点", icon: "OpenAI", baseUrl: "", apiFormat: "openai" },
{ id: "xiangong", name: "仙宫云", subtitle: "GPU 实例上的 Ollama / vLLM / ComfyUI 等 OpenAI 兼容端点", icon: "OpenAI", baseUrl: "", apiFormat: "openai" },
```

用户在「设置 → 模型服务 → 新建渠道」会看到「仙宫云」卡片，粘贴仙宫云 GPU 实例暴露的 OpenAI 兼容端点（vLLM 默认 `:8000/v1`、Ollama 默认 `:11434/v1`）+ Key 即可。能力自动映射：文本→`chat-completion`、图片→`openai-image`、视频→`newapi`、音频→`openai-audio`。完整步骤见 `docs/content/docs/overview/xiangong-cloud.md`。

### 怎么连（以 AutoDL / RunPod 为例）
1. 在云平台开一台带 GPU 的实例，部署 vLLM / ComfyUI / Text Generation WebUI，确认暴露 OpenAI 兼容端口。
2. 光宇无限画布 → 设置 → 模型服务 → 选「云端 GPU」或对应专用插件。
3. Base URL 填该实例的公网/隧道地址，填 Key（无 Key 的服务留空）。
4. 保存后做连接测试，模型目录会自动从 `/v1/models` 拉取；不支持自动发现的，在渠道里手动补模型名。

---

## 三、把「布布」接入为画布 Agent

### 本次新增：`workbuddy-agent` 插件
`plugin-packages/workbuddy-agent/`（manifest + README + docs/interface.md）：
- 复用 OpenAI Chat Completions 协议，作用域含 `agent`，即布布可在画布里以 **Agent** 身份参与：生成文/图/视频提示词、调用工具、编排工作流。
- 默认系统提示写入布布人格（务实、有主张、简洁）。
- `baseUrl` 默认占位 `https://api.workbuddy.local/v1`，需在渠道设置中替换为**真实端点**。

### 已定的接入方式（你让我「看着办」）
当前 WorkBuddy 没有可直连的公开 Agent 端点，因此布布采用**本地桥接**方案（我替你拍板）：
- 新增 `tools/workbuddy-bridge/server.mjs`：监听 `http://127.0.0.1:8787/v1`，把请求转发到环境变量 `WORKBUDDY_UPSTREAM_URL` 配置的任意 OpenAI 兼容上游（纯 Node 内置模块，无依赖）。
- `workbuddy-agent` 插件默认 `baseUrl` 改为 `http://127.0.0.1:8787/v1`。
- 把上游指向上面仙宫云实例的 vLLM/Ollama 端点，布布即可用你的 GPU 推理跑起来；以后 WorkBuddy 开放官方端点，只需把桥接上游换成官方地址即可。

这样布布现在就能用，而且直接吃你的仙宫云算力。详见 `tools/workbuddy-bridge/README.md` 与 `plugin-packages/workbuddy-agent/README.md`。

---

## 四、构建与运行（本地优先桌面端）

前置：Go 1.22+、Node 20+、bun 1.3+、Wails v2、Windows WebView2 / macOS WKWebView。

```bash
git clone <你的仓库> 光宇无限画布
cd 光宇无限画布
# 后端
cd backend && go mod download
# 前端与插件
cd ../web && bun install --frozen-lockfile
# 打包插件（新增的 workbuddy-agent 会被一起构建）
cd ../plugin-packages && bash build-packages.sh
# 桌面运行
wails dev        # 开发
wails build      # 打包
```

> Windows 上 Wails 构建需要 CGO 与 WebView2 运行时；若失败，优先检查 `wails doctor` 输出。

---

## 五、待你确认/提供
1. **仙宫云端点**：在 `tools/workbuddy-bridge/.env` 的 `WORKBUDDY_UPSTREAM_URL` 填你仙宫云实例的 `/v1` 地址（vLLM `:8000/v1` 或 Ollama `:11434/v1`）即可让布布跑起来。若需要我加「仙宫云一键部署脚本 + 自动读取实例端点」，请提供仙宫云开放 API 资料。
2. **布布端点**：已用本地桥接方案定下，默认吃你的仙宫云算力；待 WorkBuddy 有官方端点时换上游即可。
3. **改名深度**：✅ 已做。二进制名、安装目录、以及运行时数据目录均已改名为「光宇无限画布」（见第一节与第六节）。

---

## 六、运行时品牌对齐（最终补全）

安装器/二进制改名后，Go 运行时仍残留若干用户可见的 "BeefTV" 字符串，会导致「装出来叫光宇无限画布、打开却显示 BeefTV」的不一致。已统一：

| 文件 | 改动 |
|------|------|
| `backend/cmd/desktop/main.go` | Wails 窗口标题 `Title: "BeefTV"` → `"光宇无限画布"` |
| `backend/internal/appearance/types.go` | `DefaultBrandName="BeefTV"`→`"光宇无限画布"`；`DefaultBrandSlug="beeftv"`→`"guangyu"` |
| `backend/internal/desktopstorage/location.go` | `DefaultRoot()` 数据目录 `BeefTV`→`光宇无限画布`；新增一次性迁移 `LegacyDataDirName="BeefTV"`（首次启动把旧目录 `os.Rename` 到新名，保留工程与配置） |
| `backend/internal/runtimeinfo/runtimeinfo.go` | `DefaultDataDir()` 改用 `desktopstorage.DefaultRoot()`；移除不再使用的 `fmt` 导入 |
| `web/src/stores/use-appearance-store.ts` | `DEFAULT_PUBLIC_APPEARANCE.brandSlug` `"beeftv"`→`"guangyu"`；`brandStudioLabel` 回退 `"BEEF CREATIVE STUDIO"`→`"光宇无限画布"` |

**主动保留、未改的 BeefTV 残留**（属代码标识/企业品牌/测试，改名反而破坏兼容）：
- `beeftv` 包名、`cmd/beeftv` 构建目标、`BEEFTV_*` 环境变量、`.beeftv-plugin` 扩展名；
- `beefapi` 企业品牌（`service.go` 的 `hostname="BeefTV"`、`updates.beefapi.com`）；
- macOS 包名 `BeefTV.app` 及 `update-release/archive.go` 内的 zip 路径字符串；
- 全部 `_test.go` 中的 fixture 路径与断言（覆盖式测试，使用 `BEEFTV_DATA_DIR` 注入，不受默认目录改名影响）。

**未实机编译验证**：本沙箱无 Go 工具链 / WebView2 / CGO，仅做了人工审阅与无用导入清理。请在具备 Go+CGO+WebView2 的 Windows 上执行 `wails build` 实机验证，并确认迁移逻辑（`%AppData%\BeefTV` → `%AppData%\光宇无限画布`）符合预期。
