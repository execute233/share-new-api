# Design: Remove Payment System, Make Public Free Relay

日期：2026-08-16
状态：Approved（grilling 后确认）

## 背景与目标

将本项目从「付费中转站」改造为「公开免费公益中转站」：

1. **删除全部在线支付逻辑**：Stripe / epay / Creem / Waffo / WaffoPancake 充值通道、订阅支付回调、支付合规确认、支付回跳、webhook 可用性监控。
2. **保留核心免费额度机制**：兑换码（Redemption）、管理员后台充值（编辑用户额度 + AdminCompleteTopUp）、签到（Checkin）、邀请等现有额度来源全部不动。
3. **保留订阅机制但不经支付**：订阅 = 周期额度发放 + 用户组升降级；获取方式仅剩「管理员开通」与「钱包余额购买」（余额来自兑换码/后台充值）。
4. **公开运营**：注册策略保持可配置，不做任何强制改动。

非目标（不做）：

- 不引入任何新表、新抽象层、额度策略引擎。
- 不写支付表的 DROP 迁移；旧库中残留的 TopUp/SubscriptionOrder 表忽略，新库不建。
- 不动注册/认证/权限体系，不动渠道层与 relay 层（上一轮渠道删除范围之外一律不碰）。
- 不删 option 表中的存量配置行（仅删代码引用）。

## 决策记录（grilling 结论）

| 决策点 | 结论 |
|---|---|
| quota 制度 | 保留 quota；兑换码 + 后台充值是用户自助额度来源 |
| 删除范围 | 在线充值全套、订阅支付回调、支付合规/回跳、充值设置（后端 option 配置，无独立前端页面）、充值记录页 |
| 计费设置页 | `features/system-settings/billing` 为**计费配置**（billing_mode/billing_expr 模型倍率），与支付无关，**保留** |
| 订阅机制 | 保留：周期额度发放 + 组升级；获取方式 = 管理员开通 + 余额购买 |
| 余额购买订阅 | 保留（SubscriptionRequestBalancePay）；需保留 PriceAmount/Currency 作定价 |
| 后台充值 | 两者都保留：用户管理页编辑额度 + `AdminCompleteTopUp` 改造为手动充值接口（原为补单语义，依赖 TopUp 表；TopUp 表删除后改造为 userId+quota 直接加额度并写日志） |
| 存量数据 | 不写迁移删除表；TopUp/SubscriptionOrder 从 AutoMigrate 列表移除，旧表残留忽略 |
| 注册策略 | 保持可配置 |
| 兑换码扩展 | 不扩展（仍只发 quota），订阅码独立体系不做 |
| 前端文案 | 导航「充值」→「兑换」 |

## 删除清单

### Go 后端（controller）

- `controller/topup_stripe.go`
- `controller/topup_creem.go`
- `controller/topup_waffo.go`
- `controller/topup_waffo_pancake.go`
- `controller/subscription_payment_epay.go`
- `controller/subscription_payment_stripe.go`
- `controller/subscription_payment_creem.go`
- `controller/subscription_payment_waffo_pancake.go`
- `controller/payment_compliance.go`
- `controller/return_path.go`（仅被 epay 订阅回调使用，随 epay 删除成为死文件）
- `controller/payment_webhook_availability.go`
- `controller/topup.go` 内：`GetTopUpInfo`、`GetUserTopUps`、`GetAllTopUps`、`RequestEpay`、`EpayNotify`、`RequestAmount` 及其私有辅助函数（`GetEpayClient`/`getPayMoney`/`getMinTopup`/`getTopUpQuota`/`getMaxTopUpAmount`/`validateCreditedQuota`/`validateTopUpQuota`/`rejectInvalidCreditedQuota`/`rejectInvalidTopUpQuota`/`LockOrder`/`UnlockOrder`/`refCountedMutex` 等）；**`AdminCompleteTopUp` 保留并改造**为手动充值：请求体改为 `userId`+`quota`，直接给用户加额度并记录日志（不再依赖 TopUp 表）
- `controller/user.go` `TopUp`（兑换码兑换入口，保留）：移除 `IsPaymentComplianceConfirmed` 检查
- 路由 `POST /api/user/topup`（兑换码兑换）**保留**

### Go 后端（model）

- `model/topup.go` 整体删除（TopUp 表）
- `model/subscription.go`：`SubscriptionOrder` 结构体及其方法（Insert/Update/GetSubscriptionOrderByTradeNo/CompleteSubscriptionOrder/ExpireSubscriptionOrder/upsertSubscriptionTopUpTx/状态机等）删除；`SubscriptionPlan` 删除字段 `StripePriceId`、`CreemProductId`、`WaffoPancakeProductId`、`MaxPurchasePerUser`；**保留** `PriceAmount`、`Currency`（余额购买定价）与 `AllowBalancePay`（余额购买开关）；`PurchaseSubscriptionWithBalance` 改造为不写 `SubscriptionOrder`（直接扣款+建订阅+日志）；`PaymentMethodBalance`/`PaymentProviderBalance` 常量迁移至 `model/subscription.go`
- `model/main.go`：`TopUp`、`SubscriptionOrder` 从 `AutoMigrate` 与 `migrateDBFast` 列表移除（不写 DROP）

### 路由

- 删除：`/api/user/topup/info`、`/api/user/topup/self`、`/api/user/pay`、`/api/user/amount`、`/api/user/stripe/pay|amount`、`/api/user/creem/pay`、`/api/user/waffo/amount|pay`、`/api/user/waffo-pancake/amount|pay`、`/api/user/topup`（admin 充值记录列表）、`/api/user/topup/complete` 保留、`/api/stripe/webhook`、`/api/creem/webhook`、`/api/waffo/webhook`、`/api/waffo-pancake/webhook/:env`、`/api/user/epay/notify`（GET/POST）、`/api/subscription/epay/pay`、`/api/subscription/epay/notify`（GET/POST）、`/api/subscription/epay/return`（GET/POST）、`/api/payment_compliance`
- 保留：`POST /api/user/topup`（兑换码兑换）、`/api/user/topup/complete`（AdminCompleteTopUp，改造后语义为手动充值）、`/api/subscription/*` 其余链路、兑换码管理路由

### 测试

- 删除：`controller/topup_quota_limit_test.go`、`controller/payment_webhook_availability_test.go`、`controller/return_path_test.go`、`model/payment_method_guard_test.go`
- 保留：`model/subscription_auth_test.go`、`model/subscription_reset_test.go`（订阅核心链路回归）

### 依赖

- 删除并 go mod tidy：`github.com/stripe/stripe-go/v81`、`github.com/Calcium-Ion/go-epay`、`github.com/waffo-com/waffo-go`、`github.com/waffo-com/waffo-pancake-sdk-go`

### option / setting

- 删除支付相关 option 代码定义与引用（TopUpLink、Stripe 密钥、epay 配置、Waffo 配置、`operation_setting/payment_setting.go` 的 PaymentSetting/Compliance 等），option 表存量行保留；`QuotaDisplayType`（general_setting.go）保留（通用展示配置，非支付专用）
- `service/epay.go`、`service/return_path.go`、`service/waffo_pancake.go` 删除；`service/webhook.go`（通用 webhook 通知）保留
- `i18n/keys.go`：`MsgPaymentComplianceRequired`、`MsgPaymentMethodNotExists` 删除（`MsgRedeemFailed`/`MsgUserTopUpProcessing` 保留，兑换码功能使用）

## 保留清单

- 兑换码全链路：`model/redemption.go`（Redeem/CRUD/过期）、管理页、钱包兑换 tab
- 订阅核心：`SubscriptionPlan` / `UserSubscription` / `SubscriptionPreConsumeRecord`、周期额度重置（daily/weekly/monthly/custom）、组升降级、`AllowWalletOverflow`
- `SubscriptionRequestBalancePay`（余额购买订阅，扣钱包 quota，用 PriceAmount 定价）
- 订阅管理：`AdminCreateUserSubscription`（管理员开通）、`AdminBindSubscription`、`AdminListUserSubscriptions`、重置/作废/删除等管理接口
- 后台充值：用户管理页编辑额度 + `AdminCompleteTopUp`（改造为手动充值接口，UI 入口在用户详情）
- 签到、邀请、注册送额度等现有机制
- 计费链路（quota 预扣/结算/模型倍率）完全不动

## 前端改造

- 删除：`features/wallet/components/recharge-form-card.tsx`（重构为轻量兑换码卡片）、`creem-products-section.tsx`、`payment-confirm-dialog.tsx`、`creem-confirm-dialog.tsx`、`use-creem-payment.ts`、`use-waffo-payment.ts`、`use-waffo-pancake-payment.ts`、`use-topup-info.ts`、`lib/payment.ts`、`hooks/use-payment.ts` 及对应测试、充值记录管理（GetAllTopUps 接口 + 前端账单历史中的充值记录视图）、订阅购买弹窗中的支付方法选择
- **保留**：`features/system-settings/billing`（计费配置，非支付）、`system-settings/operations` 等其余设置页
- 钱包页保留：兑换码卡片（自 recharge-form-card 提取）、账单历史（数据源改为日志 API `type=LogTypeTopup`，替代已删的 `/api/user/topup/self`）、订阅计划卡片（简化为余额购买单流程）
- 订阅页保留：计划浏览 + 「我的订阅」状态 + 购买弹窗（无支付方法，余额不足提示兑换/联系管理员）
- 导航：侧边栏「充值」入口改为「兑换」
- 用户管理页：`user-quota-dialog.tsx` 已提供额度调整（add/override/subtract）即后台充值 UI，**无需新增**；`AdminCompleteTopUp` 作为手动充值 API 保留（供脚本/第三方）
- i18n：删除支付相关文案键（en/zh/zh-TW/fr/ja/ru/vi），保留兑换/订阅文案

## 验证策略

1. `go build ./...`、`go vet ./...`
2. `go test ./...`（仅允许预存不稳定：channel_affinity 时间戳碰撞）
3. 前端 `bun run typecheck`、`bun run test`、`bun run build`
4. 零残留扫描：`stripe`、`epay`、`creem`、`waffo`、`topup`（业务层）、`payment`、`return_path`、`payment_compliance` 关键词在 Go 业务代码与前端源码（locales 之外）零命中；`AdminCompleteTopUp`、兑换码、订阅关键词正常保留
5. `go mod tidy` 后 go.mod/go.sum 无上述支付依赖
6. 单次提交（含本 spec 文档）

## 风险与边界

- 余额购买订阅依赖 PriceAmount 定价：删除支付字段时必须保留 `PriceAmount`/`Currency`，否则 `SubscriptionRequestBalancePay` 无价可算（grilling 已确认）。`AllowBalancePay` 同样保留（余额购买开关，非在线支付字段）。
- `PurchaseSubscriptionWithBalance` 不再写 `SubscriptionOrder`（表已删），改为直接扣 quota + 建订阅 + 写日志。
- `AdminCompleteTopUp` 原为 TopUp 表补单语义，改造为手动充值（userId+quota）时需同步更新其前端 UI 入口与请求参数，并保持路由 `/api/topup/complete` 不变。
- `LogTypeTopup` 日志类型保留：兑换码充值记录使用该类型。
- 旧库 TopUp/SubscriptionOrder 表残留但不被任何代码引用，无运行时风险。
- 不触碰 relaykit、渠道层、认证层（前一轮工作范围外）。
