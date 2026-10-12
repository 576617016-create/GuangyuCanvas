# 布布 (WorkBuddy) 接口字段

## 协议身份

- 插件 ID：`workbuddy-agent`。
- Provider ID：`workbuddy-agent`。
- 能力：`text`（Agent 模式下可驱动工具调用）。
- 默认 Base URL：`https://api.workbuddy.local/v1`（请在渠道设置中替换为真实端点）。
- 鉴权驱动：`bearer`。
- 创建：`POST /chat/completions`。
- Agent：`POST /chat/completions`。
- 生命周期：同步响应。

## 配置字段

| 字段 | 类型 | 必填 | 含义 |
| --- | --- | --- | --- |
| `apiKey` | secret | 是 | 端点 API Key |
| `baseUrl` | string | 否 | 兼容端点地址，覆盖默认占位地址 |

## 统一字段映射

| 统一字段 | 类型 | 必填 | 上游映射 | 说明 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | `model` | 上游模型 ID。 |
| `messages` | message[] | 是 | `provider message container` | 包含历史消息和当前用户输入。 |
| `instructions` | string | 否 | `system/instructions` | 系统指令（布布人格）。 |
| `temperature` | number | 否 | `temperature` | 采样温度。 |
| `top_p` | number | 否 | `top_p` | 核采样参数。 |
| `max_tokens` | integer | 否 | `max_tokens/max_output_tokens` | 最大输出 token。 |
| `tools` | array | 否 | `tools/toolConfig` | 工具定义。 |
| `tool_choice` | object\|string | 否 | `tool_choice` | 工具选择策略。 |
| `response_format` | object | 否 | `response_format/text` | 结构化输出配置。 |
| `stream` | boolean | 否 | `stream` | 流式开关。 |

## 上游请求模板逐字段清单

| 上游位置 | 值或转换表达式 |
| --- | --- |
| `create.method` | `"POST"` |
| `create.path` | `"/chat/completions"` |
| `create.contentType` | `"application/json"` |
| `create.body.model` | `{"$ref":"request.model"}` |
| `create.body.messages` | `{"$ref":"request.messages"}` |
| `create.body.temperature` | `{"$omitEmpty":{"$ref":"request.providerOptions.chat-completion.temperature"}}` |
| `create.body.top_p` | `{"$omitEmpty":{"$ref":"request.providerOptions.chat-completion.top_p"}}` |
| `create.body.max_tokens` | `{"$omitEmpty":{"$coalesce":[{"$ref":"request.extra.max_tokens"},{"$ref":"request.providerOptions.chat-completion.max_tokens"}]}}` |
| `create.body.tools` | `{"$omitEmpty":{"$ref":"request.providerOptions.chat-completion.tools"}}` |
| `create.body.tool_choice` | `{"$omitEmpty":{"$ref":"request.providerOptions.chat-completion.tool_choice"}}` |
| `create.body.response_format` | `{"$omitEmpty":{"$ref":"request.providerOptions.chat-completion.response_format"}}` |
| `create.body.stream` | `{"$omitEmpty":{"$ref":"request.providerOptions.chat-completion.stream"}}` |
| `agent.method` | `"POST"` |
| `agent.path` | `"/chat/completions"` |
| `agent.contentType` | `"application/json"` |
| `agent.body` | `{"$merge":[{"$ref":"request.extra.agent.chatCompletion"},{"model":{"$ref":"request.model"}}]}` |

## 响应映射逐字段清单

| 映射位置 | 上游路径或转换表达式 |
| --- | --- |
| `response.status` | `"succeeded"` |
| `response.textPaths[0]` | `"choices.0.message.content"` |
| `response.textPaths[1]` | `"choices.0.text"` |
| `response.reasoningPaths[0]` | `"choices.0.message.reasoning_content"` |
| `response.usage` | `{"$ref":"response.usage"}` |
| `response.errorPaths[0]` | `"error.code"` |
| `response.messagePaths[0]` | `"error.message"` |
| `agentResponse.textPaths[0]` | `"choices.0.message.content"` |
| `agentResponse.textPaths[1]` | `"choices.0.text"` |
| `agentResponse.reasoningPaths[0]` | `"choices.0.message.reasoning_content"` |
| `agentResponse.toolCallsPath` | `"choices.0.message.tool_calls"` |
| `agentResponse.toolCallIdPaths[0]` | `"id"` |
| `agentResponse.toolCallNamePaths[0]` | `"function.name"` |
| `agentResponse.toolCallArgumentsPaths[0]` | `"function.arguments"` |

## 响应与错误

插件把上游 task/status/text/media/usage 映射为统一结果。临时媒体 URL 标记为 ephemeral，由宿主立即下载持久化。HTTP 错误、业务 code 和 error object 保持失败语义，不包装成成功。

<!-- BEEFTV_PLUGIN_MANIFEST_START -->
## Manifest 完整接口定义

见插件包内 `manifest.json`（与上文逐字段一致）。
<!-- BEEFTV_PLUGIN_MANIFEST_END -->
