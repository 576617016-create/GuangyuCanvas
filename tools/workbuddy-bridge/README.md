# 布布 (WorkBuddy) 本地桥接

把 光宇无限画布 里「布布」Agent 渠道（默认 `http://127.0.0.1:8787/v1`）转发到任意 **OpenAI 兼容**上游，例如你在**仙宫云** GPU 实例上跑的 vLLM / Ollama。

> 为什么需要桥接：当前 WorkBuddy 没有可直连的公开 Agent 端点。布布先通过这座本地桥接用你自己的 GPU 推理跑起来；等 WorkBuddy 开放官方端点，只需把上游换成官方地址即可。

## 运行

```bash
cd tools/workbuddy-bridge
cp .env.example .env          # 填入 WORKBUDDY_UPSTREAM_URL（仙宫云实例的 /v1 地址）
node server.mjs
```

启动后：

- 健康检查：`http://127.0.0.1:8787/healthz`
- Chat 兼容：`POST http://127.0.0.1:8787/v1/chat/completions`（直接转发到上游）

## 在画布里启用布布

1. 设置 → 模型服务 → 新建渠道 → 选 **「布布 (WorkBuddy)」**。
2. Base URL 填 `http://127.0.0.1:8787/v1`，Key 留空或与上游一致。
3. 在画布助手(Assistant)中使用，系统提示写入布布人格即可。

## 依赖

仅 Node.js 内置模块，无需 `npm install`。支持流式与非流式透传。
