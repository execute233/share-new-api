# Codex 伪装渠道（CodexDisguise）设计文档

- 日期：2026-08-28
- 状态：已确认（方案 A，三节设计全部通过）
- 范围：new-api 主模块新增渠道类型 `ChannelTypeCodexDisguise`（Codex伪装），参照 sub2api 的伪装能力，对外暴露标准接口与 codex 原生路径，将下游多个 codex 客户端合并为统一的 codex 会话，出站接入上游 sub2api 实例。

## 1. 背景与目标

### 1.1 现状

- 主模块已有 `ChannelTypeCodex`（57）直连渠道：单账号 OAuth 直连 chatgpt.com，仅有 originator 补缺等最小伪装（`relay/channel/codex/adaptor.go`）。
- 主模块已有 `relay/channel/sub2api`（59）：只是 newapi adaptor 的轻量包装（标准协议转发到 sub2api 实例）。
- sub2api 模块（独立 go.mod）内有完整伪装引擎：身份三元组、指纹收敛、turn-state 溯源、会话隔离、版本同步、Agent Identity 等——主模块没有这些逻辑。

### 1.2 目标

新增"Codex伪装"渠道，实现：

1. 参照 sub2api 能力，把核心伪装集 + Agent Identity 移植到主模块；
2. 对外暴露标准接口（`/v1/responses` 等，已存在）与 codex 原生路径（`/backend-api/codex/*`，需新增）；
3. 将下游多个 codex 客户端合并为统一的 codex 会话（设备+会话全收敛，thread 按下游派生）；
4. 出站接入上游 sub2api 实例（不改动 sub2api，账号由 sub2api 管理员配置）；
5. 出站走 sub2api 的 codex 原生路径，整体呈现"官方 Codex CLI 直连"形态。

### 1.3 非目标（本期不做）

- WebSocket 转发 / realtime / Live
- sub2api 的 wham 用量面板
- 修改 sub2api 模块
- 计费体系改动（沿用标准渠道 token 计费）

## 2. 整体架构

```
下游客户端（真实 Codex CLI / 任意 OpenAI 兼容客户端）
   │  入站（TokenAuth + Distribute 鉴权，new-api 自有 token）
   ▼
new-api "Codex伪装" 渠道 (ChannelTypeCodexDisguise)
   ├─ 标准路径: /v1/responses, /v1/responses/compact, /v1/alpha/search（已存在）
   ├─ codex 原生路径: /backend-api/codex/responses, /responses/compact,
   │                  /alpha/search, /models（新增路由组）
   │
   │  伪装引擎（出站前）:
   │  ① Key 解析（三种格式: sub2api api_key / OAuth auth / Agent Identity）
   │  ② 身份三元组重写（UA/originator/version 配套 + 强制统一）
   │  ③ 指纹收敛（installation/session 渠道级恒定, thread 按下游派生）
   │  ④ 会话隔离（session_id 按 new-api 维度收敛）
   │  ⑤ turn-state 溯源与剥离（按 thread 归属）
   │  ⑥ 请求体归一（store=false、instructions 注入、compact 适配）
   ▼
上游 sub2api 实例（不改动）
   │  出站: /backend-api/codex/responses 等 codex 原生路径
   │  鉴权: Bearer sub2api-api-key / Bearer OAuth access_token / AgentAssertion
   ▼
ChatGPT（sub2api 内部账号池，管理员配置）
```

## 3. 请求数据流（一次 /responses 请求）

1. **入站**：`/v1/responses`（或 codex 原生路径）→ `controller.Relay` → `relaykit` 解析为 `OpenAIResponsesRequest` → `Distribute` 按模型路由到 CodexDisguise 渠道。
2. **Key 解析**：`ParseDisguiseKey(ch.Key)` → 识别 `type`（sub2api/oauth/agent），拿到出站凭据。
3. **身份构造**：`resolveOutboundIdentity()` — 版本优先级（渠道手配 > 自动同步 > 编译期常量），UA/originator 配套。
4. **指纹收敛**：`resolveConvergedFingerprintIDs()` — installation/session 用渠道级 seed 派生；thread 用 `isolateThreadID(下游 token 维度, 客户端 session)` 派生；头+体双写（`x-codex-installation-id`、`x-codex-window-id`、`x-codex-turn-metadata`、body `client_metadata`）。
5. **turn-state 守卫**：入站 `x-codex-turn-state` 若由本渠道另一 thread 铸造 → 剥离；否则原样透传。
6. **出站**：POST `{sub2api_base}/backend-api/codex/responses`，携带全部伪装头 + 对应格式认证头 + `chatgpt-account-id`（仅 OAuth 格式带）。
7. **回程**：SSE/JSON 流原样回传下游；`x-codex-turn-state` 从上游响应捕获 → 记录铸造归属（thread → account）后回传下游。

## 4. 渠道类型与注册

| 项 | 值 |
|---|---|
| 常量 | `ChannelTypeCodexDisguise = 62`，`ChannelTypeDummy` 移至 63 |
| `ChannelBaseURLs` | 62 位补位（默认上游为空串，渠道 BaseURL 必填指向 sub2api 实例） |
| `ChannelTypeNames` | `"Codex Disguise"`（显示名"Codex伪装"） |
| 适配器注册 | `relay/relay_adaptor.go` `GetAdaptor` 加 `case constant.ChannelTypeCodexDisguise` |
| 前端 | `web/src/features/channels/constants.ts` 的 `CHANNEL_TYPES`/`TYPE_TO_KEY_PROMPT` 等补位 |

## 5. 组件清单

### 5.1 新增文件（主模块）

| 组件 | 文件 | 内容 |
|---|---|---|
| 适配器主体 | `relay/channel/codexdisguise/adaptor.go` | `Init`/`GetRequestURL`/`SetupRequestHeader`/`ConvertOpenAIResponsesRequest`/`DoRequest`/`DoResponse`；其他协议端点一律拒绝（错误文案对齐现有 codex 渠道风格） |
| Key 解析 | `relay/channel/codexdisguise/key.go` | `DisguiseKey` 结构（type/access_token/refresh_token/account_id/api_key/agent_private_key/agent_runtime_id/agent_task_id），`ParseDisguiseKey` |
| 身份三元组 | `relay/channel/codexdisguise/identity.go` | 规范 UA 构造、originator 配对、version 门槛（≥0.144.0）、强制统一收口、`PairCodexClientIdentity`（含 `(name; version)` 尾部恢复） |
| 指纹收敛 | `relay/channel/codexdisguise/fingerprint.go` | `FingerprintIDs`（installation/session/thread/window/turn）、SHA-256 确定性派生、头+体双写一致性 |
| turn-state | `relay/channel/codexdisguise/turn_state.go` | 铸造溯源表（thread→account，TTL+惰性清理）、回带剥离 |
| Agent Identity | `relay/channel/codexdisguise/agent_identity.go` | `buildAgentAssertion`、task 注册/恢复（ed25519 签名 + NaCl box 解密） |
| 版本同步 | `service/codex_disguise_version_sync.go` | 定时拉 GitHub openai/codex 最新稳定版（rust-v 前缀、每页 30、只前移、6h 间隔），写入 Setting；渠道手配/管理员手配优先 |
| 路由 | `router/relay-router.go` | 新增 `/backend-api/codex` 前缀组（responses/compact/alpha-search/models），挂 TokenAuth+Distribute |

### 5.2 复用现有机制（不新增）

- 计费：标准渠道 token 计费（倍率/模型映射沿用）
- 测试入口：渠道连通性测试 `TestAdaptor` 系列
- 多 Key：不支持（与 codex 渠道一致，`channel_info.is_multi_key` 拒绝）
- 代理：渠道 ProxyID 复用
- OAuth 刷新：复用 `service.RefreshCodexOAuthTokenWithProxy`（`service/codex_oauth.go`）与 `service.RefreshCodexChannelCredential`

## 6. 渠道配置项（扩展 `OtherSettings` JSON / 渠道表单）

| 配置 | 键 | 默认 | 说明 |
|---|---|---|---|
| 伪装开关 | `disguise_enabled` | true | 关闭则退化为纯转发（仅认证替换） |
| 指纹模式 | `fingerprint_mode` | `session` | off/device/session/full；默认 session（1 设备 1 会话 N 线程） |
| 指纹 seed | `fingerprint_seed` | 自动生成 | 渠道级恒定 UUIDv4，派生 installation/session |
| 客户端版本 | `codex_client_version` | 空=自动同步 | 手配优先，<0.144.0 拒绝 |
| 强制统一 | `enforce_identity` | true | 关闭则保留客户端自带身份（仅配套校验） |
| 上游 base URL | 渠道 BaseURL | 必填 | sub2api 实例地址 |
| Agent 自动注册 | `agent_auto_register` | true | task 失效自动重注册 |

## 7. 出站头构造顺序（不可调换）

1. URL：按 RelayMode 选 sub2api 的 codex 原生路径（responses/compact/alpha-search）
2. 认证头（按 Key 格式：Bearer api_key / Bearer OAuth access_token / AgentAssertion）
3. 白名单透传客户端头（对齐现有 `SetupApiRequestHeader` 基础 + codex 特有头）
4. turn-state 守卫（剥离异 thread 回带值）
5. originator / Accept（流式 `text/event-stream`，compact `application/json`）
6. session_id 收敛值覆盖
7. 指纹收敛头改写（与 body 同一份 IDs）
8. 终态收口：强制统一 UA/originator/version（若 `enforce_identity`）
9. `content-type` 兜底为精确 `application/json`

## 8. 错误处理

| 场景 | 处理 |
|---|---|
| Key 非 JSON / 未知 type | 本地 400，渠道测试/请求均报 `invalid disguise key` |
| 出站 401/403（OAuth） | 有 refresh_token → 自动刷新（复用现有逻辑）→ 回写渠道 Key → 重试一次 |
| 出站 401/403（Agent） | task 无效标记 → 重新注册 → 重试一次；注册失败 → 透传错误 |
| 出站 404（身份被拒） | 记录 SysError 含 UA/originator/version 三元组；不自动重试 |
| 出站 429 / server_is_overloaded | 透传给下游，渠道置短暂冷却（AutoBan 开启则按现有逻辑） |
| 上游非 2xx | 原样透传 body 与状态码（保持 OpenAI 错误格式） |
| 下游请求体不合法（instructions 非字符串等） | 本地 403 拒绝，不上游 |

## 9. 测试策略

对齐 AGENTS.md 后端测试规范（require/assert，确定性表驱动，保护真实契约）：

| 测试文件 | 覆盖 |
|---|---|
| `relay/channel/codexdisguise/adaptor_test.go` | URL 构造（三种 RelayMode + 原生路径）、头构造（三种 Key 格式认证头）、指令注入、store=false、参数剥离 |
| `relay/channel/codexdisguise/identity_test.go` | UA/originator 配对（含 `(name; version)` 尾部恢复）、version 门槛（<0.144.0 拒绝/回退）、强制统一开关 |
| `relay/channel/codexdisguise/fingerprint_test.go` | 派生确定性（同 seed 同 ID）、头+体双写一致（含内嵌 turn-metadata 同步改写）、thread 隔离不互串 |
| `relay/channel/codexdisguise/turn_state_test.go` | 铸造归属记录、异 thread 回带剥离、TTL 过期清理 |
| `relay/channel/codexdisguise/key_test.go` | 三种格式解析、缺失字段拒绝 |
| `relay/channel/codexdisguise/agent_identity_test.go` | assertion 签名/信封格式、task 注册请求体、加密 task id 解密（fixture 固定向量） |
| `service/codex_disguise_version_sync_test.go` | 版本解析/前移/缓存 TTL/手配优先 |
| 路由测试 | `/backend-api/codex/*` 挂载不冲突（参照现有路由测试风格） |

## 10. 里程碑

1. 常量/类型/注册/前端补位（可编译、可创建渠道）
2. Key 解析 + 身份三元组（+ 单测）
3. 指纹收敛 + turn-state（+ 单测）
4. Agent Identity（+ 单测）
5. 路由组 + 适配器接线（端到端可转发）
6. 版本同步 + 模型同步（可选）
7. 全量测试跑通 + 文档

## 11. 关键参考

- 本仓库 `docs/codex/technical-report.md`、`docs/codex/disguise-guide.md`（伪装原理与头规范）
- sub2api 实现：`sub2api/backend/internal/service/openai_codex_identity.go`、`openai_codex_fingerprint.go`、`openai_codex_turn_state.go`、`openai_agent_identity.go`、`openai_codex_version_sync_service.go`、`openai_gateway_forward.go`
- 主模块现有：`relay/channel/codex/adaptor.go`（直连渠道，可作风格参照）、`relay/channel/sub2api/`、`router/relay-router.go`
- 上游行为知识库：docs/codex 文档 §13（404 规则、降载、配额）