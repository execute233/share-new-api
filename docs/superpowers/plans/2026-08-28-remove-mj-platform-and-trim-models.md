# 2026-08-28: 删除 Midjourney 平台 + 瘦身内置模型 — 实施计划

分支：dev。参考 spec：`docs/superpowers/specs/2026-08-28-remove-mj-platform-and-trim-models-design.md`（不提交）。每 Task 验证后提交（--no-gpg-sign，显式路径 add）。

## Task 1: 模型列表瘦身（4 适配器）

- `relay/channel/openai/constant.go`：ModelList 只留 gpt-5*（含补充 gpt-5.5、gpt-5.6-sol/terra/luna）+ codex-auto-review + gpt-image-2
- `relay/channel/claude/constants.go`：只留 opus-4-6、sonnet-4-6、opus-4-7、opus-4-8
- `relay/channel/gemini/constant.go`：只留 9 个 gemini-3*/3.1*
- `relay/channel/codex/constants.go`：不变
- 验证：`go build ./...`、`go vet ./...`、`go test ./relay/... ./controller/... -count=1`
- 提交：`chore(relay): trim built-in model lists to current generations`

## Task 2: 后端核心链删 MJ

整删 7 文件：model/midjourney.go、dto/midjourney.go、constant/midjourney.go、setting/midjourney.go、relay/mjproxy_handler.go、service/midjourney.go、controller/midjourney.go

清理：router/relay-router.go（/mj 组 + registerMjRouterGroup）、router/api-router.go（/api/mj 组）、middleware/distributor.go（/mj/ 分支）、middleware/auth.go（midjourney-proxy 回退）、middleware/utils.go（abortWithMidjourneyMessage）、controller/relay.go（RelayMidjourney）、controller/system_task_handlers.go（poll handler）、controller/misc.go（enable_drawing/mj_notify + GetMidjourney）、controller/model.go（mj 收集循环）、model/option.go（6 项设置）、model/system_task.go、model/main.go（表注册+迁移）、common/constants.go（DrawingEnabled）、constant/task.go（TaskPlatformMidjourney）、relay/constant/relay_mode.go（**iota `_` 占位** + Path2RelayModeMidjourney + /mj 分支）、relay/common/relay_info.go（case）、service/error.go（2 wrapper）、service/log_info_generate.go（GenerateMjOtherInfo）、setting/ratio_setting/model_ratio.go（17 条 mj_* 倍率）

- 验证：`go build ./...` 循环修编译错、`go vet ./...`、`go test ./... -count=1`
- 提交：`feat: remove Midjourney task platform from backend`

## Task 3: relaykit + i18n keys

- `relaykit/types/relay_format.go` 删 RelayFormatMjProxy；`relaykit/types/error.go` 删 ErrorTypeMidjourneyError
- `i18n/keys.go` 删 MsgDistributorInvalidMidjourney；`i18n/locales/{en,zh-CN,zh-TW}.yaml` 删对应行
- 验证：`$env:GOWORK='off'; go build ./...` + `go test ./...`（relaykit）；根模块 `go build ./...`
- 提交：`chore(relaykit): remove midjourney relay format and error type`

## Task 4: 前端 usage-logs 删绘图日志

整删：drawing-logs-filter-bar.tsx、columns/drawing-logs-columns.tsx、dialogs/image-dialog.tsx、dialogs/prompt-dialog.tsx

清理：types.ts、constants.ts（MJ 映射段 + LOG_CATEGORY_LABELS）、lib/mappers.ts、lib/index.ts、lib/utils.ts（drawing 分支）、lib/filter.ts、lib/columns.ts、api.ts、index.tsx、section-registry.tsx、usage-logs-table.tsx、usage-logs-mobile-card.tsx

- 验证：`bunx vitest run src/features/usage-logs`、`bunx oxlint`（涉及文件）、typecheck
- 提交：`feat(web): remove drawing logs from usage logs`

## Task 5: 前端系统设置/sidebar/杂项删 MJ

- 整删：system-settings/content/drawing-settings-section.tsx
- 清理：system-settings/types.ts、content/index.tsx、content/section-registry.tsx、hooks/use-sidebar-data.ts、hooks/use-sidebar-config.ts、maintenance/config.ts、maintenance/sidebar-modules-section.tsx、profile/sidebar-modules-card.tsx、channels/lib/model-categories.ts、system-info/system-tasks-panel.tsx、layout/footer.tsx、lib/legacy-route.ts
- 验证：vitest（channels/system-settings 相关）、oxlint、typecheck
- 提交：`feat(web): remove midjourney from settings, sidebar, and routing`

## Task 6: i18n 键清理

- 加载 i18n-translate skill；用脚本从 7 语言删 MJ 专属键（Drawing logs、Filter by MjProxy task ID、History of MjProxy-style image tasks.、Enable drawing features、Save drawing settings、MjProxy 相关 ~16 键）；通用词键逐个确认无引用后删
- static-keys.ts 同步删
- 验证：`bun run i18n:sync` 报告 0 missing；`node find-missing-keys` 全过
- 提交：`chore(web): remove midjourney i18n keys`

## Task 7: 测试 + 文档

- `service/test_helpers_test.go`：删 Midjourney AutoMigrate + DELETE FROM midjourneys
- `web/src/lib/legacy-route.test.ts`：更新 MJ 映射断言
- README.md ×6、constant/README.md、docs/learn ×3、docs/spec/outbound-proxy-module.md：清理 MJ 行
- 验证：`go test ./... -count=1`、`bunx vitest run`、typecheck
- 提交：`chore: update docs and tests after midjourney removal`

## Task 8: 全套验证

- 根模块：`go build ./...`、`go vet ./...`、`go test ./... -count=1`
- relaykit：`$env:GOWORK='off'; go build ./...; go test ./...`
- web：`bun run typecheck`、`bunx vitest run`、`bunx oxlint`（涉及文件）、`bun run i18n:sync` 报告
- 汇报完成（spec/plan 不提交）