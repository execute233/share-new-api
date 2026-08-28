# Codex 请求伪装指南

> 面向需要对接 OpenAI ChatGPT 内部接口（`chatgpt.com/backend-api/codex`）的代理网关/中转实现。本文总结 sub2api 的经验：**哪些头、哪些体字段、什么顺序、什么坑**。逐条做到，请求即可稳定通过上游的身份与风控判定。

---

## 1. 伪装总原则

1. **同源性**：同一请求的所有身份载体必须指向同一个"虚拟客户端"。UA、originator、version、installation_id、session_id、thread_id、turn_id、turn-metadata、请求体 client_metadata、认证断言——任何两个载体自相矛盾，就是代理链的破绽。
2. **贴近真实**：真实 Codex CLI 发什么，你就发什么；真实客户端不发的东西（多余的环境噪声头、跨账号回带的旧值），你也不发。
3. **版本永远最新**：陈旧版本在上游容量紧张时会被优先降载（HTTP 200 + 流内 `server_is_overloaded`）。
4. **只在一个地方定义身份**：UA 形态、版本号、originator 配对逻辑各收敛到一处，禁止多处拼装（拼装漂移 = 自相矛盾）。

---

## 2. 推理面必带请求头

目标端点（OAuth 账号）：

```
POST https://chatgpt.com/backend-api/codex/responses
Host: chatgpt.com                      ← 必须用 req.Host，不能用 Header.Set
```

| 头 | 值 | 必带 |
|---|---|---|
| `User-Agent` | `codex-tui/0.146.0 (Ubuntu 22.4.0; x86_64) xterm-256color` | ✅ |
| `Originator` | `codex-tui`（**必须与 UA 首段配套**，否则 404） | ✅ |
| `Version` | `0.146.0`（**≥ 0.144.0**，且 = UA 版本段；低于门槛 404） | ✅ |
| `Authorization` | `Bearer <access_token>` 或 `AgentAssertion <...>` | ✅ |
| `Accept` | `text/event-stream`（流式）/ `application/json`（compact） | ✅ |
| `OpenAI-Beta` | `responses=experimental` | ✅ |
| `Content-Type` | `application/json` | ✅ |
| `session_id` | 隔离后的会话标识（见 §6） | ✅ |
| `x-codex-installation-id` | 账号级恒定 UUIDv4 | ✅（收敛开启时） |
| `x-codex-window-id` | `thread_id:0` | ✅（收敛开启时） |
| `x-codex-turn-metadata` | 内嵌 JSON（见 §5） | 客户端携带则必改 |
| `x-codex-turn-state` | 客户端回带原值；**换号后必须剥离** | 条件 |

**禁止带**：`x-stainless-*`、浏览器 UA、`accept-encoding` 自定义值、`x-forwarded-for` 伪造值等环境噪声头——白名单以外的头一律不透传（sub2api 的做法是显式白名单，宁可少发不可多发）。

---

## 3. 凭据面（auth.openai.com）头规则

换 Token / 刷新 / whoami / Agent task 注册：

- 只带 `User-Agent` + `Originator`（**不发 `version`**）——真实 codex-rs 的 `default_headers()` 在该面只发这两个；
- UA/originator 与推理面同源（同一解析链），否则"登录时 A 客户端、推理时 B 客户端"会被识别。

---

## 4. 请求体规范

```jsonc
{
  "model": "gpt-5-codex",                    // 或 gpt-5.x-codex / codex-mini 等
  "instructions": "<真实 Codex base prompt>", // 必带且非空；真实客户端用的是官方 base prompt
  "input": [
    {"type": "message", "role": "user", "content": "..."}
  ],                                          // 字符串会被包装成数组
  "store": false,                             // ChatGPT 内部接口强制 false
  "stream": true,                             // 非 compact 强制 true
  "prompt_cache_key": "...",                  // 与 session_id 一致（默认值场景）
  "client_metadata": {
    "x-codex-installation-id": "<uuid4>",
    "session_id": "<uuid4>",
    "thread_id": "<uuid4>",
    "turn_id": "<uuid4 v7 随机>",
    "x-codex-window-id": "<thread_id>:0",
    "x-codex-turn-metadata": "{\"installation_id\":\"...\",\"session_id\":\"...\",\"thread_id\":\"...\",\"turn_id\":\"...\",\"window_id\":\"...\",\"turn_started_at_unix_ms\":<ms>}"
  }
}
```

要点：

- `instructions` 缺失/空/非字符串：本地拒绝（403）或注入官方 base prompt。sub2api 内嵌了真实 Codex CLI 各模型的 base prompt（GPT-5-Codex / 5.1 / 5.2 / 5.5）；
- 顶层不支持字段（如 `previous_response_id` 的某些组合、`store=true`）会被删除/归一；
- compact 路径（`/responses/compact`）：**unary JSON**，不带 `stream`/`store`，`Accept: application/json`。

---

## 5. 设备指纹：头与体必须双写一致

同一请求中，以下载体**必须**携带完全相同的 ID 集合（特别是随机的 `turn_id`——头里一个值、体里另一个值 = 实锤代理）：

| 载体 | 位置 |
|---|---|
| `x-codex-installation-id` | 请求头 + body `client_metadata` |
| `session_id` / `session-id` | 请求头（两种写法都要） + body `client_metadata` |
| `thread_id` | 请求头 + body `client_metadata` |
| `turn_id` | body `client_metadata`（+ 内嵌 turn-metadata） |
| `x-codex-window-id` | 请求头 + body `client_metadata` |
| `x-codex-turn-metadata` | 请求头 JSON 字符串 + body `client_metadata` 内嵌 JSON 字符串 |

推荐收敛策略（多人共享账号时）：

- **默认关闭**（额度缩水风险，上游配额判定不可观测）；
- `session` 模式最接近正常用户：1 设备 + 1 会话 + N 线程（线程按客户端真实 session 派生，模拟 spawn 子代理）；
- `full` 最激进（1 设备 1 会话 1 线程），可能被上游判定异常；
- ID 用 SHA-256 确定性派生（UUIDv4 格式），同一种子永远同值；`turn_id` 每次请求随机。

---

## 6. 会话隔离与回合状态

- **多租户共享账号必须隔离 session**：不同下游用户在 session 标识里混入各自唯一 ID（sub2api 用 `xxhash64("k<apiKeyID>:"+raw)`），否则跨用户会话在上游碰撞；
- **`x-codex-turn-state` 是上游铸造的回合 blob**，客户端会在同一回合后续请求中回带。代理必须记录"哪个账号铸造的"：failover 换号后，客户端回带的 blob 与当前账号的出站身份（installation/session/thread）矛盾，**必须剥离**再出站。同账号回放则原样保留；
- 只剥离不注入（注入是给无法回带回合状态的 Claude 兼容桥用的，`/responses` 真实 Codex 客户端不需要）。

---

## 7. 认证：Agent Assertion 格式

如果使用 Agent Identity（推荐，绕开 access token 共享风控）：

```
注册: POST https://auth.openai.com/api/accounts/v1/agent/{runtime_id}/task/register
      body: {"timestamp": "<RFC3339>", "signature": <ed25519(runtime_id + ":" + timestamp) base64>}
      → 返回 task_id（或 encrypted_task_id，需用派生 X25519 私钥 box 解密）

请求: Authorization: AgentAssertion <base64url(JSON{
        "agent_runtime_id": "<runtime_id>",
        "task_id": "<task_id>",
        "timestamp": "<RFC3339>",
        "signature": <ed25519(runtime_id + ":" + task_id + ":" + timestamp) base64>
      })>
```

- 断言每次请求重新签名（时间戳新鲜）；
- 任务失效（401/403 + task 无效标记）→ 自动重新注册再重试；
- 注册必须加锁防止并发重复注册。

---

## 8. 常见被拒原因与修复速查

| 现象 | 根因 | 修复 |
|---|---|---|
| 404 | `originator` 与 UA 首段错配 | 用 `PairCodexClientIdentity` 重写 UA 首段或 originator 使其配套 |
| 404 | `version` < 0.144.0 | 版本门槛校正；低于门槛就整体回退官方当前版本 |
| HTTP 200 + 流内 `server_is_overloaded` | 陈旧/第三方身份被优先降载 | 强制统一出站身份为规范 Codex TUI；版本自动同步最新 |
| 403 | codex 请求缺有效 `instructions` | 注入官方 base prompt（按模型选择）或本地拒绝 |
| 401/403 周期性 | Agent task 过期 | 自动注册 + 重试 + 恢复 |
| 额度异常缩水 | 指纹收敛过激 / 多设备多会话 | 关闭收敛或调低模式；账号级恒定 installation_id |
| 上游 400 | body 携带内部接口不支持的字段 | 删除/归一（store=false、input 数组化等） |
| 会话连续性断裂 | 换号后旧 turn-state 回带 | 溯源表剥离异账号回带值 |

---

## 9. 出站头构造顺序（不可调换）

1. URL（OAuth → `chatgpt.com/backend-api/codex/responses`；API Key → `api.openai.com/v1/responses`）
2. 认证头
3. `Host: chatgpt.com` + `chatgpt-account-id`
4. 客户端头白名单透传
5. turn-state 守卫（剥离异账号值）
6. originator / Accept / version（compact 才带 version 兜底；凭据面不带 version）
7. session/conversation 隔离值覆盖
8. 自定义 UA / ForceCodexCLI（可选）
9. **指纹收敛头改写**（与 body 同一份 IDs）
10. **终态收口**：强制统一 UA/originator/version（三元组同源自洽）
11. content-type 兜底 → 账号头覆写 → beta 特性补注

其中第 9、10 步必须在所有其他 UA/身份改写**之后**执行——收口点是最后发言权。

---

## 10. 维护清单（上游规则会变）

- **版本门槛与 UA 形态**：跟随官方 CLI 发版节奏维护（sub2api 每 6h 从 `openai/codex` GitHub releases 自动同步，tag 前缀 `rust-v`）；
- **官方客户端集合**：新官方/合作客户端（如 codex_sdk_ts、codex_vscode_copilot）出现时同步补入 UA 前缀与 originator 精确集合；
- **实测验证**：每个关键行为（404 条件、降载优先级、额度判定）以上游实际响应为准，用 `codex-auto-review` 轻量模型做连通性/配额探针；
- **失败闭环**：4xx/5xx 响应要能区分"可重试的瞬时错误"与"身份问题"——身份问题先修身份再重试，否则会反复污染账号。

---

## 11. 合规声明

本指南用于理解代理网关的协议适配原理。将第三方流量伪装成官方客户端接入 OpenAI 服务可能违反 OpenAI 服务条款，使用前请评估法律与合规风险，并对接入账号的封禁风险有充分预期。请勿用于绕过订阅限制或损害任何服务提供方权益的场景。