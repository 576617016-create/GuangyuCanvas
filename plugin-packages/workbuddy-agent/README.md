# 布布 (WorkBuddy) Agent 接入

把「布布」(WorkBuddy AI 助手) 接入 光宇无限画布 的画布 Agent 体系，复用 OpenAI Chat Completions 线协议。

本插件由 光宇无限画布 Contributors 维护；不表示与任何服务商存在隶属、授权或合作关系。

## 它能做什么

- 在画布里让布布以 **Agent** 身份参与创作：生成文字、图片/视频提示词，调用工具，编排工作流。
- 指向任意 **OpenAI 兼容** 的 chat/agent 端点（自建网关、兼容服务、或 WorkBuddy 暴露的兼容端点）。

## 快速开始

1. 启动本地桥接（见仓库 `tools/workbuddy-bridge`）：把上游 `WORKBUDDY_UPSTREAM_URL` 指向你的仙宫云 vLLM/Ollama 端点（或任意 OpenAI 兼容服务）。
2. 设置 → 模型服务 → 新建渠道，选择「布布 (WorkBuddy)」。
3. Base URL 填 `http://127.0.0.1:8787/v1`（本地桥接），Key 与上游一致或留空。
4. 在画布助手(Assistant)中使用，系统提示可写入布布人格：
   > 你是布布，光宇无限画布里务实、有主张、简洁的 AI 助手。先动手做、必要时再澄清；不堆砌客套话。在画布中协助用户组织创意、生成与迭代文/图/视频/音频素材。

> 为何走桥接：当前 WorkBuddy 没有可直连的公开 Agent 端点。布布先通过本地桥接用你自己的 GPU（如仙宫云）推理跑起来；待 WorkBuddy 开放官方端点，把桥接上游换成官方地址即可。

## 兼容边界

- 该包只声明 OpenAI Chat Completions profile（文本 / Agent 工具调用）。
- 若端点另提供图片/视频生成接口，请配合 `openai-images` / `newapi` 等协议插件分别接入。

完整接口字段见 [docs/interface.md](docs/interface.md)。
