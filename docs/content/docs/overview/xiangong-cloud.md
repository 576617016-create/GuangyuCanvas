---
title: 仙宫云 GPU 接入指南
description: 在 光宇无限画布 中通过仙宫云 GPU 实例运行云端推理
---

# 仙宫云 GPU 接入指南

[仙宫云](https://www.xiangongyun.com/) 是高性价比 GPU 算力租赁平台（RTX 4090 低至约 2 元/小时），支持一键部署 ComfyUI、Stable Diffusion、Ollama、vLLM 等镜像。光宇无限画布把仙宫云实例上暴露的 **OpenAI 兼容端点**作为「云端 GPU」渠道接入，无需把数据上传到第三方模型服务。

## 接入流程

1. **租实例**：在仙宫云控制台选择 GPU（如 RTX 4090）+ 镜像。
   - 跑大模型/对话：选 **Ollama** 或 **vLLM** 相关镜像。
   - 跑图/视频：选 **ComfyUI** / **Stable Diffusion** 镜像（这类通常走 ComfyUI 工作流，对应插件 `autodl-comfyui` 的思路，或自定义 newapi 端点）。
2. **启动并拿到地址**：实例运行后，镜像会把服务开在某个端口：
   - vLLM：默认 `http://<实例地址>:8000/v1`（OpenAI 兼容）
   - Ollama：默认 `http://<实例地址>:11434/v1`（OpenAI 兼容）
   - 在实例详情里确认「应用端口 / WebUI 端口」与可访问地址（公网或隧道）。
3. **在画布里建渠道**：
   - 设置 → 模型服务 → 新建渠道 → 选 **「仙宫云」** 预设。
   - Base URL 填上面的 `.../v1` 地址；若镜像要求 Key 则填，否则留空。
   - 保存后做连接测试，模型目录会自动从 `/v1/models` 拉取；不支持自动发现的，手动补模型名（如 `qwen2.5:72b`、`deepseek-r1`）。
4. **能力自动映射**：文本→`chat-completion`、图片→`openai-image`、视频→`newapi`、音频→`openai-audio`。图/视频生成本身由 ComfyUI/SD 镜像提供，若其端点不是标准 OpenAI 形状，可在渠道里指定对应协议（如 `newapi`）。

## 让「布布」用仙宫云当大脑

布布（WorkBuddy）Agent 默认指向本地桥接 `http://127.0.0.1:8787/v1`（`plugin-packages/workbuddy-agent`）。把这座桥接的上游配成仙宫云上的 vLLM/Ollama 端点，布布就能直接用你的 GPU 推理：

```bash
# tools/workbuddy-bridge/.env
WORKBUDDY_UPSTREAM_URL=http://<仙宫云实例地址>:8000/v1
WORKBUDDY_UPSTREAM_KEY=        # 无 Key 的服务留空
PORT=8787
```

然后运行桥接（见 `tools/workbuddy-bridge/README.md`），布布即可在画布里以 Agent 身份工作。待 WorkBuddy 开放官方 Agent 端点后，只需把上游换成官方地址。

## 计费与注意

- 仙宫云**按量秒级计费**，用完记得在实例里「关机/销毁」，否则持续扣费。
- 模型与输出先存到云存储或下载到本地，实例销毁不丢数据。
- 端口与地址以仙宫云实例详情页为准；若只暴露 WebUI 不暴露 API 端口，需自行在实例内用 `ollama serve` / `vllm serve` 起 OpenAI 兼容服务并开放端口。
