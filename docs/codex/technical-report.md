# sub2api Codex 请求伪装技术报告

> 对象：`sub2api/`（OpenAI/Anthropic 订阅转 API 网关）
> 范围：本文档从路由、出站身份、设备指纹、请求体变换、认证、会话状态、版本同步、限流快照等层面，完整拆解 sub2api 如何把第三方流量"伪装"成官方 Codex CLI 请求，并成功打进 OpenAI 的 ChatGPT 内部接口（`chatgpt.com/backend-api/codex`）。
> 所有引用行号基于 2026-08 快照；代码随仓库演进，请以实际文件为准。

---

## 1. 总体架构：两层伪装

sub2api 的 Codex 伪装分为两个独立平面，各自携带不同的身份：

| 平面 | 出站目标 | 携带身份 | 说明 |
|---|---|---|---|
| **凭据面（auth）** | `auth.openai.com/api/accounts/*` | `User-Agent` + `originator`（**无 `version`**） | 换 Token / 刷新 / whoami / Agent task 注册。真实 codex-rs 在该面只发这两个头（`login/default_client.rs` 的 `default_headers()`），`version` 门槛只存在于推理面 |
| **推理面（inference）** | `chatgpt.com/backend-api/codex/responses`（OAuth）或 `api.openai.com/v1/responses`（API Key） | `User-Agent` + `originator` + `version` + 设备指纹 + `x-codex-turn-*` | 真正的对话/推理请求 |

凭据面与推理面的身份必须同源（`CodexCanonicalAuthIdentity` 与推理链共用 `resolveCodexOutboundIdentity`），否则上游能看出"登录时是一个客户端、推理时是另一个客户端"的矛盾。

```
用户请求 (任意 OpenAI 兼容客户端)
  → sub2api 网关 (apiKeyAuth 鉴权)
    → 账号调度 (sticky session / failover)
      → Forward 或 Passthrough 分支
        → 身份三元组重写 (UA / originator / version)
        → 设备指纹收敛 (installation/session/thread/turn/window)
        → 请求体 normalize (store=false / instructions / compact)
        → 出站到 chatgpt.com/backend-api/codex/responses
```

---

## 2. 入站路由与端点映射

路由注册位于 `backend/internal/server/routes/gateway.go`。Codex 相关入口全部收敛到同一批 handler：

| 入站路径 | 方法 | Handler | 上游映射 |
|---|---|---|---|
| `/v1/responses`、`/v1/responses/*subpath` | POST | `OpenAIGatewayHandler.Responses` | 追加 `/responses` 子路径到上游 |
| `/responses`、`/responses/*subpath` | POST | 同上（无 `/v1` 前缀入口） | 同上 |
| `/backend-api/codex/responses`、`/responses/*subpath` | POST | 同上（**Codex CLI 原生路径**） | 同上 |
| `/backend-api/codex/responses` 子路径如 `/compact` | POST | `guardResponsesSubpath` 放行 | 子路径原样拼接上游（`appendOpenAIResponsesRequestPathSuffix`） |
| `/v1/responses`、`/responses`、`/backend-api/codex/responses` | GET | `ResponsesWebSocket` | WS 握手到上游（见 §11） |
| `/backend-api/codex/alpha/search` | POST | `AlphaSearch` | `chatgpt.com/backend-api/codex/alpha/search` |
| `/backend-api/codex/models` | GET | `CodexModels` | 返回 ChatGPT Codex manifest 格式（Codex CLI 刷新模型选择器用） |
| `/backend-api/codex/realtime/calls` | POST | `Live` | 实时语音/推理会话 |
| `/backend-api/codex/realtime/calls/:call_id` | GET | `LiveSideband` | 会话旁路 |
| `/responses/input_tokens` | POST | `ResponsesInputTokens` | 原生 input-token 计数端点 |

要点：

- `/responses/*subpath` 的子路径会被**原样转发**到上游同名端点，因此入口有 `guardResponsesSubpath`（routes/gateway.go:161）调用 `IsForwardableOpenAIResponsesRequestPath` 拒绝畸形子路径（`..%2f`、反斜杠等），防止把不可控片段拼进上游 URL（见 `openai_gateway_request_body.go` 的 `rawOpenAIResponsesRequestPathSuffix` / `sanitizedUpstreamPathSuffix`）。
- `openAIResponsesRequestPathSuffix` 提取 `/responses` 之后的子路径，其中 `/compact` 及其子路径被单独识别（`isOpenAIResponsesCompactPath`），走 unary JSON 协议。
- 请求经中间件链：`bodyLimit → clientRequestID → opsErrorLogger → endpointNorm → apiKeyAuth → compositeTarget → requireGroupAnthropic`——鉴权始终是 **sub2api 自己的 API Key**，与上游无关。

---

## 3. 出站身份三元组（伪装核心）

文件：`backend/internal/service/openai_codex_identity.go`、`backend/internal/pkg/openai/request.go`

### 3.1 三个头必须自洽

上游对 `/backend-api/codex` 有严格的客户端身份校验（issue #3901，2026-07 实测）：

1. **`originator` 必须与 `User-Agent` 首段（第一个 `/` 前的 client 名）配套**，错配直接 **404**；
2. **`version` 头如果携带且低于 `0.144.0` 直接 404**（`codexUpstreamMinVersion`）；不带则无门槛；
3. **`version` 必须等于 UA 版本段**（同一版本声明的两个出口，漂移即矛盾）。

因此代码定义了一个 `codexOutboundIdentity` 三元组，三个字段永远从同一个解析链派生：

```go
type codexOutboundIdentity struct {
    userAgent  string // {originator}/{version} (Ubuntu 22.4.0; x86_64) xterm-256color
    originator string // codex-tui 等
    version    string // 0.146.0 等
}
```

### 3.2 规范身份与 UA 形态

```go
codexCLIUserAgentSuffix = " (Ubuntu 22.4.0; x86_64) xterm-256color"
codexCLIUserAgent       = "codex-tui/" + codexCLIVersion + codexCLIUserAgentSuffix
codexCLIVersion         = "0.146.0"   // 编译期兜底，必须跟随官方 CLI 当前发布版
codexDefaultOriginator  = "codex-tui" // 默认 originator
```

UA 形态刻意对齐真实 Codex TUI：`{client}/{version} ({OS} {OS_version}; {arch}) {terminal}`。注释明确说明：**缺少 OS/架构/终端后缀的形态易被上游指纹识别为非官方客户端**。

### 3.3 版本号校验与重建

- `NormalizeCodexClientVersion`：正则 `^[0-9]+(\.[0-9]+){1,3}(-[0-9A-Za-z.]+)?$`（允许 `0.146.0` 与 `0.147.0-alpha.4` 两种官方形态）、最长 64 字符。非法值一律拒绝——因为该值会被拼进 UA 与 version 头，不能放任意字节。
- `resolveCodexOutboundIdentity(candidateUA)` 的解析链：
  1. 取规范 UA（面板配置 → 自动同步 → 编译期常量）；
  2. `PairCodexClientIdentity`：UA 首段是官方 originator → 直接配对；否则取 UA 尾部括号组 `(name; version)` 的 name（应对 `CODEX_INTERNAL_ORIGINATOR_OVERRIDE` 场景，如 `cccc/0.142.0 ... (codex-tui; 0.142.0)`）→ 用尾部 name 重写首段；都失败 → 整体回退默认官方身份；
  3. 版本段**一律用当前生效版本重建**（`SetCodexUserAgentVersion` 同时更新首段与尾部括号组的版本），管理员填写的历史 UA 不会被钉死在陈旧版本上；
  4. 版本低于门槛 → 回退编译期常量。

### 3.4 强制统一收口

`enforceCodexIdentityHeadersWithUA`（openai_codex_identity.go:219）是所有出站路径共用的收口点：

- **默认（强制统一）**：UA / originator / version 一律改写为网关规范身份，客户端自报身份完全不参与构造。原因：上游在容量紧张时按客户端身份分优先级降载（被降载的请求拿到 HTTP 200 + 流内 `server_is_overloaded`），统一出口确保没有请求带着第三方或陈旧身份出站；
- **关闭强制（`gateway.disable_codex_identity_enforcement`）**：退回 `pairCodexIdentityHeaders` 收口语义——保留客户端真实身份，仅保证 originator 与最终 UA 配套、version 不低于门槛；
- 仅对携带 `originator` 的请求生效：compat 桥接等非 ChatGPT 内部接口路径会显式删除 originator，不应被补回；
- `ensureCodexIdentityHeaders` 负责补齐缺失的身份头 + `OpenAI-Beta: responses=experimental`。

### 3.5 官方客户端指纹知识库

`backend/internal/pkg/openai/request.go` 内置了对官方客户端的完整取证：

- **官方 UA 前缀集合**：`codex_cli_rs/`、`codex-tui/`、`codex_vscode/`、`codex_vscode_copilot/`、`codex_app/`、`codex_chatgpt_desktop/`、`codex_atlas/`、`codex_exec/`、`codex_sdk_ts/`；
- **`Codex ` 家族前缀**（Codex Desktop 等，保留空格避免退化为裸 `codex` 宽松匹配）；
- **官方 originator 精确集合**（镜像 codex-rs 的 `is_first_party_originator`），拒绝 `evil-codex_` 之类伪造；
- **UA 尾部兜底**：`(name; version)` 括号组恢复被 override 的真实 client。生产审计（10GB / 23 天）显示 `originator=cccc` 的真实 codex-tui 占全 openai 流量 5.3%，无此兜底会全部误拒。

---

## 4. 设备/会话指纹收敛

文件：`backend/internal/service/openai_codex_fingerprint.go`

### 4.1 动机

多人共享同一 OAuth 账号时，每个用户的 Codex 客户端携带各自不同的 `installation_id` / `session_id` / `thread_id`，上游据此判定设备数和会话数——设备/会话越多越像"批发中转站"。收敛模式把这些标识改写为**账号级恒定值**。

### 4.2 四种收敛模式（账号 extra 配置，显式 opt-in）

| 模式 | 上游看到的形态 | 收敛内容 |
|---|---|---|
| `off`（默认） | 每个客户端原样 | 不做任何改写 |
| `device` | 1 台设备 + 多会话 | 仅收敛 `installation_id` 为账号级恒定值 |
| `session` | 1 设备 + 1 会话 + N 线程 | `installation_id` + `session_id` 收敛，`thread_id` 按客户端原始 session-id 确定性派生（最接近正常用户 spawn 子代理） |
| `full` | 1 设备 + 1 会话 + 1 线程 | 全部收敛（最激进） |

> 实测背景（代码注释引用 issue #5555 / #5556 / #5582 / #5610）：新账号开收敛后额度缩水、回退 v0.1.173 恢复——上游配额判定策略不可观测，故默认 `off` 保持兼容安全一侧。

### 4.3 ID 派生算法

`deriveStableUUIDv4(seed)`：SHA-256 种子 → 取前 16 字节 → 强制 UUIDv4 版本/变体位 → 格式化。同一种子永远同一值，无随机性。

```
installation_id = 管理员配置 device_id  (优先)
                ∥ deriveStableUUIDv4("sub2api:codex-install-id:v2:" + seed)
session_id      = deriveStableUUIDv4("sub2api:codex-session-id:v2:" + seed)
thread_id       = deriveStableUUIDv4("sub2api:codex-thread-id:v2:" + seed + ":" + clientSessionID)
                （session 模式按客户端真实 session-id 派生；full 模式 = session_id）
turn_id         = UUIDv7 随机（每次请求不同）
window_id       = thread_id + ":0"
turn_started_at_unix_ms = 请求时刻
```

seed 来自账号 extra 的 `codex_fingerprint_seed`（必须是规范 UUIDv4，创建账号时生成）。

### 4.4 改写载体（头 + 体双写，必须一致）

**出站请求头**（`applyCodexFingerprintHeaders`）：

| 头 | 值 |
|---|---|
| `x-codex-installation-id` | 收敛 installation_id |
| `session-id` 与 `session_id` | 收敛 session_id（两种写法都改） |
| `thread-id` | 收敛 thread_id |
| `x-client-request-id` | 收敛 thread_id |
| `x-codex-window-id` | window_id |
| `x-codex-turn-metadata` | 内嵌 JSON 重写 `installation_id`/`session_id`/`thread_id`/`turn_id`/`window_id`/`turn_started_at_unix_ms` |

**请求体**（`applyCodexFingerprintClientMetadata` / 透传热路径 `applyCodexFingerprintClientMetadataRaw` 用 gjson/sjson 做字节级外科手术）：

```json
"client_metadata": {
  "x-codex-installation-id": "...",
  "session_id": "...",
  "thread_id": "...",
  "turn_id": "...",
  "x-codex-window-id": "...",
  "x-codex-turn-metadata": "{...内嵌同一套 IDs...}"
}
```

- `prompt_cache_key` 仅在可证明是 body session 默认值时改写为收敛 session_id（`shouldRewriteCodexFingerprintPromptCacheKey`）；
- 内嵌 `x-codex-turn-metadata` JSON 字符串必须同步改写，避免 flat 与 embedded 暴露两套身份（`rewriteClientMetadataEmbeddedTurnMetadata` / `rewriteCodexTurnMetadataFields`）。

### 4.5 一致性机制

- 收敛 ID 一次解析（`resolveCodexFingerprintIDs`），通过 gin context 暂存（`stageCodexFingerprintIDs`），**头改写与体改写共享同一份 IDs**——`turn_id` 等随机字段在两侧必须逐字一致；
- `stagedCodexFingerprintIDs` 校验暂存 IDs 属于当前 attempt 的账号，failover 换号后旧 IDs 不残留（`stageCodexFingerprintIDs(c, nil)` 无条件覆写）。

---

## 5. 请求体变换

文件：`backend/internal/service/openai_gateway_request_body.go`、`openai_codex_transform.go`、`openai_gateway_passthrough.go`

### 5.1 OAuth 透传体 normalize（`normalizeOpenAIPassthroughOAuthBody`）

1. 删除 ChatGPT internal API 不支持的顶层 Responses 参数（`openAIChatGPTInternalUnsupportedFields`）；
2. `input` 为字符串 → 包成 `[{"type":"message","role":"user","content":...}]`；`input` 为对象 → 包成数组；
3. 非 compact：强制 `store=false`、`stream=true`（ChatGPT 内部接口的行为收敛）；
4. compact：删除 `store` 与 `stream`（unary JSON 协议）。

### 5.2 `instructions` 合成

- Codex 模型（`isOpenAICodexModel`：model 串含 `codex`）且请求缺 `instructions` → 注入 `defaultCodexSynthInstructions(model)`；
- 注入的是**真实 Codex CLI 的 base prompt**：`CodexBaseInstructionsForModel` 按模型选择内嵌 prompt（`instructions.txt` 为 GPT-5-Codex base，另有 gpt-5.1/5.2/5.5 专属版本），使合成请求在提示词层面贴近真实 Codex 行为；空则回退 `"You are a helpful coding assistant."`；
- 若 `instructions` 存在但非字符串或纯空白 → **本地 403 拒绝**（`detectOpenAIPassthroughInstructionsRejectReason`），避免把坏体打到上游。

### 5.3 compact 路径

- `/responses/compact` 子路径：模型经 `resolveOpenAICompactForwardModel` 映射后写入 body，`Accept: application/json`、强制补 `version` 头、`session_id` 使用隔离值；
- reasoning effort 归一化（`normalizeOpenAICodexCompactReasoningEffort`，GPT-5.6 系）。

### 5.4 其他清洗

- 空 base64 图片输入剔除（`sanitizeEmptyBase64InputImagesInOpenAIBody`）；
- 显式 null 的 tool Schema type 修正（`sanitizeOpenAIResponsesToolParameterTypes`）；
- DeepSeek 无状态 Responses 端点：强制 `store=false`、清除 `previous_response_id`；
- Responses client tools（`custom` / `tool_search` 等）适配为上游可理解的形态（`adaptOpenAIResponsesClientTools`，仅 API Key 账号）。

---

## 6. 认证面伪装

文件：`backend/internal/service/openai_agent_identity.go`

三种认证模式，全部收敛在 `buildOpenAIAuthenticationHeaders`：

| 模式 | Authorization 头 | 凭据 |
|---|---|---|
| OAuth Bearer | `Bearer <access_token>` | ChatGPT 登录获取的 access token |
| PAT | `Bearer <pat>` | Codex PAT（`create-from-codex-pat` 导入） |
| **Agent Identity** | `AgentAssertion <base64url(JSON)>` | Ed25519 私钥 + runtime_id + task_id |

### 6.1 Agent Identity 机制（最有价值的"伪装"）

1. **账号凭据**：`agent_private_key`（PKCS#8 编码的 Ed25519）、`agent_runtime_id`、`task_id`；
2. **Task 注册**（`registerAgentIdentityTask`，首次使用/失效时自动执行）：
   - `POST https://auth.openai.com/api/accounts/v1/agent/{runtime_id}/task/register`
   - body：`{"timestamp": RFC3339, "signature": ed25519(runtime_id + ":" + timestamp)}`
   - 响应返回明文 `task_id` 或 `encrypted_task_id`（用派生 X25519 公钥做 NaCl box 非对称解密，密钥由 Ed25519 seed 经 SHA-512 派生）；
   - 注册有进程级互斥锁（`agentIdentityTaskLocks`）防止并发重复注册；
3. **请求断言**（`buildAgentAssertion`）：
   ```
   payload    = runtime_id + ":" + task_id + ":" + RFC3339(timestamp)
   signature  = ed25519(payload)
   envelope   = {"agent_runtime_id", "task_id", "timestamp", "signature"}
   header     = "AgentAssertion " + base64url(JSON(envelope))
   ```
4. **Task 失效恢复**：上游返回 401/403 且 body 标记 task 无效时，`recoverAgentIdentityTask` 重新注册并重试（`forwardOpenAIPassthrough` 循环中的 `agentTaskRecoveryTried` 分支）；
5. 换 Token / 刷新凭据时也使用同源身份（`ApplyCodexCanonicalAuthIdentity`，只带 UA + originator，不带 version）。

### 6.2 账号影子与凭据解析

- 影子账号（`quota_dimension=spark` 等）通过 `resolveCredentialAccount` 解析到母账号取凭据，出站身份仍按影子账号的调度语义；
- 多账号 failover：凭据面失败视为凭据故障，触发换号。

---

## 7. 会话状态与回合连续性

文件：`backend/internal/service/openai_codex_turn_state.go`、`openai_gateway_scheduling.go`

### 7.1 会话隔离（防跨用户碰撞）

`isolateOpenAISessionID(apiKeyID, raw) = xxhash64("k<apiKeyID>:" + raw)` 输出 16 位 hex。不同 API Key 的用户即使原始 session_id 相同，上游看到的也不同。`conversation_id` 同步使用隔离值。

### 7.2 `x-codex-turn-state` 溯源与剥离

- 上游在响应头铸造不透明 blob `x-codex-turn-state`，Codex 客户端在同一回合的后续请求中原样回带（从 SSE / compact JSON / WS 握手三种响应中捕获）；
- blob 是在**出站身份**（含指纹收敛后的 installation/session/thread）下铸造的，同账号回放自洽；
- **跨账号回放**（failover 换号后客户端仍回带旧账号的 blob）是代理链独有、真实 Codex 永远不会产生的矛盾信号 → sub2api 维护"会话 → 铸造账号"溯源表（TTL 1h，每 256 次写入清扫一次），出站守卫 `guardOpenAICodexTurnStateEcho` 在回带值已知由其他账号铸造时**剥离**；
- 只剥离不注入：`/responses` 路径的客户端是真实 Codex，会自行回带（注入是 Claude 兼容桥的专属行为）。

### 7.3 Sticky Session

`BindStickySession`：`sessionHash → accountID` 绑定，TTL 默认 1h（可配置），保证同一会话尽量落同一账号，避免频繁换号暴露。

---

## 8. 版本自动同步

文件：`backend/internal/service/openai_codex_version_sync_service.go`

- 每 6h 从 GitHub `openai/codex` 仓库拉取最新稳定 release（tag 前缀 `rust-v`，过滤无关组件 tag；每页 30 条——0.145.0 与 0.146.0 之间隔着 20+ 个 alpha）；
- 只向前推进（不降级）；写入 `SettingKeyOpenAICodexClientVersionSynced`（本服务独占）；
- **优先级链**：管理员面板手填 `SettingKeyOpenAICodexClientVersion` > 自动同步值 > 编译期常量 `0.146.0`；
- 目的：让出站规范身份始终跟随官方客户端发版，避免"版本陈旧 → 上游优先降载"。

---

## 9. 限流配额快照（读侧伪装验证）

文件：`backend/internal/service/openai_gateway_usage.go`

上游在响应头回传 Codex 配额：

- `x-codex-primary-*`（7 天窗口：`used-percent` / `reset-after-seconds` / `window-minutes`）
- `x-codex-secondary-*`（5 小时窗口）
- `x-codex-primary-over-secondary-limit-percent`（溢出比）

`ParseCodexRateLimitHeaders` 解析为 `OpenAICodexUsageSnapshot`，用于后台展示/诊断、429 处理与配额自动暂停。探测请求使用 `codex-auto-review` 模型（轻量、不消耗主配额），并携带与真实转发完全相同的规范身份（`account_usage_service.go:830` 起）。

---

## 10. 客户端识别与访问控制（codex_cli_only）

文件：`backend/internal/pkg/openai/engine_fingerprint_signal.go`、`backend/internal/service/openai_client_restriction_detector.go`

- 账号可开启 `codex_cli_only`：只放行官方 Codex 客户端（`IsCodexOfficialClientRequestStrict`：UA 前缀精确匹配 + 引擎指纹信号门），其他客户端 403；
- 引擎指纹信号（`EngineFingerprintSignal`）：默认种子只勾 `x-codex-*` 头族，管理员可配置信号集（UA、originator、session 头等）与命中策略（全部/任一）；
- 拒绝时记录请求头/体摘要（`appendCodexCLIOnlyRejectedRequestFields`），prompt_cache_key 只记 SHA-256。

---

## 11. WebSocket 转发

- `GET /v1/responses` → `ResponsesWebSocket`，支持三种上游传输：`http_sse`、`responses_websockets`（v1）、`responses_websockets_v2`；
- WSv2 入站模式（`OpenAIWSIngressMode`）：`off` / `ctx_pool` / `passthrough` / `http_bridge`；
- WS 握手同样携带完整 Codex 身份（`Host: chatgpt.com`、originator、UA、version、指纹头），并受 `x-codex-turn-state` 溯源守卫约束；
- 决策链：`OpenAIWSProtocolResolver.Resolve(account)`（全局配置 → 账号开关 → 模式），WS 入站请求只允许走 WS 上游，禁止 HTTP→WS 协议混用。

---

## 12. 出站请求构造顺序（关键）

`buildUpstreamRequest`（openai_gateway_forward.go:1063）与 `buildUpstreamRequestOpenAIPassthrough`（openai_gateway_passthrough.go:461）的**头构造顺序不可调换**：

1. 目标 URL：OAuth → `chatgpt.com/backend-api/codex/responses`；API Key → `api.openai.com/v1/responses`（或自定义 base，DeepSeek 无 `/v1` 前缀）；
2. 认证头（Bearer / AgentAssertion）；
3. `Host: chatgpt.com`（必须用 `req.Host` 而非 Header.Set）+ `chatgpt-account-id` / `x-openai-fedramp`；
4. 白名单透传客户端头（`openaiAllowedHeaders`：`accept-language`、`content-type`、`conversation_id`、`user-agent`、`originator`、`session_id`、`x-codex-beta-features`、`x-codex-installation-id`、`x-codex-turn-state`、`x-codex-turn-metadata`、`x-codex-window-id`、`responses-lite`；passthrough 另有 `accept`、`openai-beta`）；
5. `guardOpenAICodexTurnStateEcho`（剥离异账号回带）；
6. compat 桥接：删 `OpenAI-Beta`/`originator`；否则设置 `originator`；
7. compact：`Accept: application/json` + `version`；非 compact：`Accept: text/event-stream`；
8. `session_id` / `conversation_id` 用隔离值覆盖客户端值；
9. 账号自定义 UA（可选）→ `ForceCodexCLI` 强制规范 UA（可选）；
10. **指纹收敛头改写**（`applyStagedCodexFingerprintHeaders`，与 body 同一份 IDs）；
11. **终态收口**（`enforceCodexIdentityHeadersWithUA`）：强制统一 UA/originator/version；
12. `content-type` 兜底 → 账号级头覆写（API Key 账号）→ `x-codex-beta-features` 会话级补注 → routing hint。

---

## 13. 上游行为知识库（伪装有效性的依据）

代码注释与实测记录的上游规则（2026-07/08）：

| 上游行为 | 触发条件 | 代码处理 |
|---|---|---|
| 404 | `originator` 与 UA 首段不配套（issue #3901） | `PairCodexClientIdentity` 强制配套 |
| 404 | `version` 携带且 < 0.144.0（issue #3901） | `codexUpstreamMinVersion` 门槛 + 重建 |
| HTTP 200 + 流内 `server_is_overloaded` | 容量紧张时按客户端身份分优先级降载（陈旧/第三方身份优先被丢） | 强制统一出站身份 |
| 额度缩水 | 指纹收敛过激（#5555/#5556/#5582） | 默认 off，收敛可配置 |
| 401/403 + task 无效 | Agent task 过期/失效 | 自动重新注册 + 重试 |
| 403 | codex 请求缺有效 `instructions` | 本地注入默认 instructions 或拒绝 |
| 配额判定 | 设备/会话/线程数（安装指纹） | 指纹收敛降噪 |

---

## 14. 关键文件索引

| 文件 | 职责 |
|---|---|
| `backend/internal/service/openai_codex_identity.go` | 身份三元组解析、校验、强制统一收口 |
| `backend/internal/pkg/openai/request.go` | UA/originator 配对、官方客户端知识库 |
| `backend/internal/service/openai_codex_fingerprint.go` | 设备/会话指纹收敛（4 模式、头+体双写） |
| `backend/internal/service/openai_gateway_request_body.go` | 请求体 normalize、compact、路径守卫 |
| `backend/internal/service/openai_gateway_passthrough.go` | OAuth 透传转发分支 |
| `backend/internal/service/openai_gateway_forward.go` | 主转发分支 + 出站请求构造 |
| `backend/internal/service/openai_agent_identity.go` | Agent Identity 断言/注册/恢复 |
| `backend/internal/service/openai_codex_turn_state.go` | turn-state 溯源与剥离 |
| `backend/internal/service/openai_codex_version_sync_service.go` | 版本自动同步 |
| `backend/internal/service/openai_gateway_usage.go` | Codex 配额头解析与快照 |
| `backend/internal/server/routes/gateway.go` | 路由注册与子路径守卫 |
| `backend/internal/pkg/openai/constants.go` | 模型清单、Codex base instructions |
| `backend/internal/service/openai_ws_forwarder*.go` | WS 转发 |
| `backend/internal/service/account_usage_service.go` | 配额探针（codex-auto-review） |

---

*本报告基于 sub2api 仓库代码静态分析整理，用于理解代理网关的请求构造原理。实际使用请遵守 OpenAI 服务条款与当地法律法规。*