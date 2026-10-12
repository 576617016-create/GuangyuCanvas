// 布布 (WorkBuddy) 本地桥接
// ------------------------------------------------------------
// 把 光宇无限画布 的「布布」Agent 渠道（默认 http://127.0.0.1:8787/v1）
// 转发到你配置的任意 OpenAI 兼容上游（例如仙宫云上的 vLLM / Ollama）。
//
// 用法：
//   cp .env.example .env   # 填入 WORKBUDDY_UPSTREAM_URL 等
//   node server.mjs
//
// 依赖：仅 Node.js 内置模块，无需 npm install。
// ------------------------------------------------------------

import http from "node:http";
import https from "node:https";
import { URL } from "node:url";
import { readFileSync } from "node:fs";

// 极简 .env 加载（避免引入第三方依赖）
function loadEnv() {
  try {
    const text = readFileSync(new URL("./.env", import.meta.url), "utf8");
    for (const line of text.split("\n")) {
      const m = line.match(/^\s*([A-Z0-9_]+)\s*=\s*(.*)\s*$/i);
      if (m && !(m[1] in process.env)) process.env[m[1]] = m[2];
    }
  } catch {
    /* 没有 .env 就用系统环境变量 */
  }
}
loadEnv();

const PORT = Number(process.env.PORT || 8787);
const UPSTREAM = (process.env.WORKBUDDY_UPSTREAM_URL || "").replace(/\/$/, "");
const UPSTREAM_KEY = process.env.WORKBUDDY_UPSTREAM_KEY || "";

if (!UPSTREAM) {
  console.error("[workbuddy-bridge] 未配置 WORKBUDDY_UPSTREAM_URL，布布将无法真正推理。");
  console.error("[workbuddy-bridge] 示例：WORKBUDDY_UPSTREAM_URL=http://<仙宫云实例>:8000/v1");
}

const upstream = UPSTREAM ? new URL(UPSTREAM) : null;
const transport = upstream?.protocol === "https:" ? https : http;

function proxy(req, res) {
  if (!upstream) {
    res.writeHead(502, { "content-type": "application/json" });
    res.end(JSON.stringify({ error: { code: "no_upstream", message: "WORKBUDDY_UPSTREAM_URL 未配置" } }));
    return;
  }

  const targetPath = req.url && req.url.startsWith("/") ? req.url : "/";
  const options = {
    protocol: upstream.protocol,
    hostname: upstream.hostname,
    port: upstream.port || (upstream.protocol === "https:" ? 443 : 80),
    path: targetPath,
    method: req.method,
    headers: { ...req.headers, host: upstream.host },
  };
  delete options.headers["content-length"]; // 由我们重新计算
  if (UPSTREAM_KEY) options.headers["authorization"] = `Bearer ${UPSTREAM_KEY}`;

  const upstreamReq = transport.request(options, (upRes) => {
    res.writeHead(upRes.statusCode || 502, upRes.headers);
    upRes.pipe(res);
  });

  upstreamReq.on("error", (err) => {
    console.error("[workbuddy-bridge] 上游错误:", err.message);
    if (!res.headersSent) res.writeHead(502, { "content-type": "application/json" });
    res.end(JSON.stringify({ error: { code: "upstream_error", message: err.message } }));
  });

  req.pipe(upstreamReq);
}

const server = http.createServer((req, res) => {
  // 本地开发允许跨域
  res.setHeader("access-control-allow-origin", "*");
  res.setHeader("access-control-allow-headers", "*");
  res.setHeader("access-control-allow-methods", "GET,POST,OPTIONS");
  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }
  // 健康检查
  if (req.url === "/healthz") {
    res.writeHead(200, { "content-type": "application/json" });
    res.end(JSON.stringify({ ok: true, upstream: UPSTREAM || null }));
    return;
  }
  proxy(req, res);
});

server.listen(PORT, "127.0.0.1", () => {
  console.log(`[workbuddy-bridge] 监听 http://127.0.0.1:${PORT}/v1`);
  console.log(`[workbuddy-bridge] 上游: ${UPSTREAM || "(未配置)"}`);
});
