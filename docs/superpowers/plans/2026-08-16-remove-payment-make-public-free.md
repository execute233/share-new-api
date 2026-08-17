# Remove Payment System (Public Free Relay) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除全部在线支付逻辑（Stripe/epay/Creem/Waffo/Pancake 充值、订阅支付、合规、回跳），保留兑换码、管理员充值、签到、订阅（管理员开通+余额购买），将中转站改为公开免费公益形态。

**Architecture:** 纯删除重构（与渠道删除轮同风格）。Go 后端删除 model/controller/setting/service 支付文件，`AdminCompleteTopUp` 改造为手动充值 API（userId+quota），`PurchaseSubscriptionWithBalance` 不再写订单表；前端删除充值表单/支付弹窗，钱包页改为兑换卡片+账单历史（日志数据源）+订阅卡片。不写任何 DROP 迁移（旧表残留忽略，新库不建）。

**Tech Stack:** Go 1.22+ / Gin / GORM v2；React 19 / TypeScript / Bun / i18next

## Global Constraints

- 数据库三兼容（SQLite/MySQL/PostgreSQL）：不用任何数据库特有语法；本次不新增表、不写 DROP 迁移。
- 所有 JSON 操作走 `common.Marshal`/`common.Unmarshal` 等 wrapper，不直接调 `encoding/json`。
- 计费安全不变量：新 `ManualCreditQuota` 必须含 quota 上限保护（int32 饱和），禁止裸转换。
- 兑换码全链路（model/redemption.go、POST /api/user/topup、兑换码管理页）**保留不动**。
- 订阅核心（SubscriptionPlan/UserSubscription/SubscriptionPreConsumeRecord/周期重置/组升降级/余额购买）**保留**；`AllowBalancePay`/`PriceAmount`/`Currency` 字段保留。
- `features/system-settings/billing` 计费设置页（billing_expr 倍率等）**保留**；仅删其中的支付 section。
- 前端 i18n：所有新增/保留文案用 `t('...')`；本次只删键不新增键。
- 提交信息风格 `refactor(channel): ...` 类：本次用 `refactor(payment): ...`。
- Windows 环境：findstr 代替 grep；git 提交加 `--no-gpg-sign`；Go 文件改动后 `gofmt -w`。
- `git add -u`（勿用 `-A`，避免误加 .codegraph/、docs/learn/、opencode.json 等 untracked 文件）。

---

### Task 1: model 层清理（删 TopUp 表、SubscriptionOrder、常量迁移、ManualCreditQuota）

**Files:**
- Delete: `model/topup.go`
- Delete: `model/payment_method_guard_test.go`
- Modify: `model/subscription.go`（删 SubscriptionOrder 块与支付字段、改造 PurchaseSubscriptionWithBalance、迁入 PaymentMethodBalance/PaymentProviderBalance 常量）
- Create: `model/user_quota.go`（ManualCreditQuota）
- Modify: `model/main.go:261-311, 314-397`（AutoMigrate/migrateDBFast 列表）
- Modify: `common/constants.go:231-234`（TopUpStatus 常量）
- Modify: `model/log.go:254-273`（RecordTopupLog）

**Interfaces:**
- Produces: `model.ManualCreditQuota(userId int, quota int) error`（T2 的 AdminCompleteTopUp 使用；原子加额度+上限保护+缓存同步+LogTypeTopup 日志）
- Produces: `model.PaymentMethodBalance = "balance"`、`model.PaymentProviderBalance = "balance"`（迁入 subscription.go，T2 不再引用旧位置）

- [ ] **Step 1: 删除 model/topup.go 与 payment_method_guard_test.go**

```bash
git rm model/topup.go model/payment_method_guard_test.go
```

注意：`model/topup.go` 内 `creditTopUpQuota`/`ValidateTopUpQuotaCapacity`/`topUpQuotaMaxCurrent` 的**逻辑**（int32 上限原子检查）要在 Step 4 的 `ManualCreditQuota` 中复用，删除前先通读该文件结尾的 `creditTopUpQuota`（model/topup.go:87-117）与 `topUpQuotaMaxCurrent`（:58-63）。

- [ ] **Step 2: 清理 model/subscription.go 的 SubscriptionOrder 与支付字段**

删除以下块（用编辑器按行删除）：
- `ErrSubscriptionOrderNotFound`/`ErrSubscriptionOrderStatusInvalid`（:37-38）
- `type SubscriptionOrder struct {...}`（:214-250，含 Insert/Update 方法）
- `GetSubscriptionOrderByTradeNo`（:241-250）
- `CompleteSubscriptionOrder`（:569-647）与 `upsertSubscriptionTopUpTx`（:649-683）
- `ExpireSubscriptionOrder`（:685 至下一个非订单函数之前）
- `SubscriptionPlan` 字段：`StripePriceId`、`CreemProductId`、`WaffoPancakeProductId`、`MaxPurchasePerUser`（:168-173）；**保留** `PriceAmount`/`Currency`（:153-154）与 `AllowBalancePay`（:163）

删除前先 `findstr /n /c:"SubscriptionOrder" model\subscription.go` 与 `findstr /n /c:"StripePriceId" model\subscription.go` 确认每个引用位置；若 `ExpireSubscriptionOrder` 与其他函数交错（如购买次数检查 `checkSubscriptionPurchaseLimitTx` 在附近），只删订单状态机函数本身，**订阅创建/重置/查询函数保留**。

- [ ] **Step 3: 改造 PurchaseSubscriptionWithBalance（不再写订单）**

在 `model/subscription.go` 的 `PurchaseSubscriptionWithBalance`（:755-845）中删除订单创建段（:804-820，即 `now := ...` 到 `tx.Create(order)` 结束），保留：plan 校验（:766-783）、扣款（:785-797）、`CreateUserSubscriptionFromPlanTx`（:799）、事务外缓存/日志（:830-844）。删除后 `tradeNo`/`now`/`order` 变量不再存在，`time`/`fmt` import 若无其他使用则移除。

在文件顶部（Subscription 相关常量附近）添加：

```go
const (
	PaymentMethodBalance   = "balance"
	PaymentProviderBalance = "balance"
)
```

- [ ] **Step 4: 新建 model/user_quota.go（ManualCreditQuota）**

创建 `model/user_quota.go`：

```go
package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
)

// ManualCreditQuota 管理员手动充值：原子增加用户额度并强制 int32 上限。
// 条件 UPDATE 保证并发下不超 MaxQuota-1。
func ManualCreditQuota(userId int, quota int) error {
	if userId <= 0 {
		return errors.New("无效的用户 id")
	}
	if quota <= 0 || quota >= common.MaxQuota {
		return errors.New("充值额度无效")
	}
	maxCurrentQuota := common.MaxQuota - 1 - quota
	if maxCurrentQuota < 0 {
		return errors.New("充值额度超出系统可表示范围")
	}
	result := DB.Model(&User{}).
		Where("id = ? AND quota <= ?", userId, maxCurrentQuota).
		Update("quota", gorm.Expr("quota + ?", quota))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		syncCreditUserQuotaCache(userId, quota, "manual topup")
		RecordLog(userId, LogTypeTopup, logger.LogQuotaFormat())
		return nil
	}
	var count int64
	if err := DB.Model(&User{}).Where("id = ?", userId).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("用户不存在")
	}
	return errors.New("用户额度已达上限")
}
```

- [ ] **Step 5: 修正 Step 4 的日志行（无占位符）**

Step 4 中 `RecordLog(userId, LogTypeTopup, logger.LogQuotaFormat())` 是占位，改为真实文案（参考 model/redemption.go:185 的写法）：

```go
RecordLog(userId, LogTypeTopup, fmt.Sprintf("管理员手动充值 %s", logger.LogQuota(quota)))
```

并补充 `"fmt"` import。

- [ ] **Step 6: 更新 model/main.go 迁移列表**

`model/main.go:261-295`（AutoMigrate）删除 `&TopUp{},`（:274）与 `&SubscriptionOrder{},`（:284）；`model/main.go:314-353`（migrateDBFast）删除 `{&TopUp{}, "TopUp"},`（:334）与 `{&SubscriptionOrder{}, "SubscriptionOrder"},`（:344）。**不写 DROP 迁移**。

- [ ] **Step 7: 删除 common.TopUpStatus 常量与 RecordTopupLog**

`common/constants.go:231-234` 删除 `TopUpStatusPending/Success/Failed/Expired` 四个常量。`model/log.go:254-273` 删除 `RecordTopupLog` 函数（删除前 `findstr /s /m /c:"RecordTopupLog" *.go` 确认零引用）。

- [ ] **Step 8: 构建验证**

```bash
gofmt -w model/user_quota.go model/subscription.go model/main.go common/constants.go model/log.go
go build ./model/... ./common/... 
```

Expected: PASS。若编译错误，检查被删符号是否仍有引用（`findstr /s /n /c:"TopUp" /c:"SubscriptionOrder" model\*.go common\*.go` 逐个清理，SubscriptionPlan 相关保留项除外）。

- [ ] **Step 9: 运行 model 测试**

```bash
go test ./model/... -count=1
```

Expected: PASS（除预存 channel_affinity 不稳定项外）。若 `subscription_reset_test.go`/`subscription_auth_test.go` 引用被删符号，按测试语义修复（它们测订阅核心链路，应不依赖订单）。

- [ ] **Step 10: Commit**

```bash
git add -u
git commit --no-gpg-sign -m "refactor(payment): remove TopUp model and subscription order layer"
```

---

### Task 2: controller 层清理（删支付控制器、改造 AdminCompleteTopUp）

**Files:**
- Delete: `controller/topup_stripe.go`、`controller/topup_creem.go`、`controller/topup_waffo.go`、`controller/topup_waffo_pancake.go`
- Delete: `controller/subscription_payment_epay.go`、`controller/subscription_payment_stripe.go`、`controller/subscription_payment_creem.go`、`controller/subscription_payment_waffo_pancake.go`
- Delete: `controller/payment_compliance.go`、`controller/return_path.go`、`controller/payment_webhook_availability.go`
- Delete: `controller/topup_quota_limit_test.go`、`controller/payment_webhook_availability_test.go`、`controller/return_path_test.go`
- Modify: `controller/topup.go`（瘦身为 AdminCompleteTopUp）
- Modify: `controller/user.go:1346-1377`（TopUp 兑换入口去合规检查）

**Interfaces:**
- Consumes: `model.ManualCreditQuota(userId, quota)`（Task 1）
- Produces: `POST /api/user/topup/complete` 请求体 `{user_id: int, quota: int}`（T3 不改路由，T5 前端不使用该接口）

- [ ] **Step 1: 删除支付控制器文件**

```bash
git rm controller/topup_stripe.go controller/topup_creem.go controller/topup_waffo.go controller/topup_waffo_pancake.go controller/subscription_payment_epay.go controller/subscription_payment_stripe.go controller/subscription_payment_creem.go controller/subscription_payment_waffo_pancake.go controller/payment_compliance.go controller/return_path.go controller/payment_webhook_availability.go controller/topup_quota_limit_test.go controller/payment_webhook_availability_test.go controller/return_path_test.go
```

- [ ] **Step 2: 瘦身 controller/topup.go 并改造 AdminCompleteTopUp**

将 `controller/topup.go` 整体替换为：

```go
package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type AdminManualTopupRequest struct {
	UserId int `json:"user_id"`
	Quota  int `json:"quota"`
}

// AdminCompleteTopUp 管理员手动充值接口（原为支付补单，无支付后改造为直接加额度）
func AdminCompleteTopUp(c *gin.Context) {
	var req AdminManualTopupRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.Quota <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.ManualCreditQuota(req.UserId, req.Quota); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
```

删除文件中的其余全部内容（GetTopUpInfo、EpayRequest、RequestEpay、EpayNotify、LockOrder/UnlockOrder、GetUserTopUps、GetAllTopUps 等）。

- [ ] **Step 3: controller/user.go 兑换入口去合规检查**

`controller/user.go:1346-1377` 的 `TopUp` 删除第 1347-1350 行（`if !operation_setting.IsPaymentComplianceConfirmed() {...}`）。保留锁逻辑与 `model.Redeem` 调用。若 `operation_setting` import 变为未使用，移除。

- [ ] **Step 4: 构建验证**

```bash
gofmt -w controller/topup.go controller/user.go
go build ./...
```

Expected: PASS。若失败，`findstr /s /n /c:"GetEpayClient" /c:"LockOrder" /c:"GetTopUpInfo" /c:"PaymentReturnURL" controller\*.go` 清理残留引用。

- [ ] **Step 5: 运行 controller 测试**

```bash
go test ./controller/... -count=1
```

Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add -u
git commit --no-gpg-sign -m "refactor(payment): remove payment controllers, rework AdminCompleteTopUp as manual credit API"
```

---

### Task 3: 路由 + setting + service + common + 后端 i18n 清理

**Files:**
- Modify: `router/api-router.go`（删支付路由）
- Delete: `setting/payment_creem.go`、`setting/payment_stripe.go`、`setting/payment_waffo.go`、`setting/payment_waffo_pancake.go`
- Delete: `setting/operation_setting/payment_setting.go`、`setting/operation_setting/payment_setting_old.go`（PayAddress/EpayKey/Price/MinTopUp/PayMethods 所在）
- Delete: `service/epay.go`、`service/waffo_pancake.go`
- **保留**: `service/return_path.go` 与 `service/return_path_test.go`（`PaymentReturnURL` 被 service/quota.go:286,340 的额度不足通知使用，非支付特有——只拼接站点 URL）
- Modify: `common/constants.go:18`（TopUpLink）
- Modify: `i18n/keys.go`（MsgPaymentComplianceRequired/MsgPaymentMethodNotExists）与 `i18n/locales/en.yaml`、`i18n/locales/zh-CN.yaml`、`i18n/locales/zh-TW.yaml`

**Interfaces:**
- Consumes: 无（纯删除）
- Produces: 保留路由 `POST /api/user/topup`（兑换）与 `POST /api/user/topup/complete`（手动充值）

- [ ] **Step 1: 删除 router/api-router.go 支付路由**

删除行：
- :52-57（`/stripe/webhook`、`/creem/webhook`、`/waffo/webhook`、`/waffo-pancake/webhook/:env`）
- :72-73（`/user/epay/notify` GET/POST）
- :95-96（`/user/topup/info`、`/user/topup/self`）
- :98-106（`/user/pay`、`/user/amount`、`/user/stripe/pay|amount`、`/user/creem/pay`、`/user/waffo/amount|pay`、`/user/waffo-pancake/amount|pay`）
- :130（`/user/topup` admin 充值记录列表）
- :157（`/subscription/epay/pay`）
- :181-184（`/subscription/epay/notify` GET/POST、`/subscription/epay/return` GET/POST）
- :190（`/payment_compliance`）

**保留**：:97（`POST /user/topup` 兑换）、:131（`/user/topup/complete`）。

- [ ] **Step 2: 删除 setting/service 支付文件**

```bash
git rm setting/payment_creem.go setting/payment_stripe.go setting/payment_waffo.go setting/payment_waffo_pancake.go setting/operation_setting/payment_setting.go setting/operation_setting/payment_setting_old.go service/epay.go service/waffo_pancake.go
```

删除前确认：`setting/operation_setting/payment_setting.go` 内 `config.GlobalConfig.Register("payment_setting", ...)` 删除后，前端计费设置页不再读取这些 option（T5 会删前端 payment section）。`setting/operation_setting` 中 `general_setting.go` 的 QuotaDisplayType 保留（通用展示配置，非支付专用）。

- [ ] **Step 3: 删除 common.TopUpLink 与后端 i18n 键**

`common/constants.go:18` 删除 `var TopUpLink = ""`（删除前 `findstr /s /m /c:"TopUpLink" *.go` 确认零引用）。

`i18n/keys.go:146` 删除 `MsgPaymentMethodNotExists`；:155 删除 `MsgPaymentComplianceRequired`。同步删除 `i18n/locales/en.yaml`、`i18n/locales/zh-CN.yaml`、`i18n/locales/zh-TW.yaml` 中对应键（`payment.method_not_exists`、`payment.compliance_required`）。**保留** `MsgRedeemFailed`（`redeem.failed`）与 `MsgUserTopUpProcessing`（`user.topup_processing`）。

- [ ] **Step 4: 删除支付依赖并验证**

```bash
go mod tidy
go build ./... && go vet ./...
```

Expected: go.mod 移除 `stripe-go`、`go-epay`、`waffo-go`、`waffo-pancake-sdk-go`。若 `go build` 报错，用 findstr 找残留引用（`findstr /s /n /c:"payment_creem" /c:"StripeMinTopUp" /c:"GetCallbackAddress" /c:"WaffoPancake" /c:"operation_setting.PayMethods" controller\*.go setting\*.go service\*.go router\*.go common\*.go i18n\*.go`）逐个清理。

- [ ] **Step 5: 零残留扫描**

```bash
findstr /s /m /c:"stripe" /c:"creem" /c:"waffo" /c:"epay" /c:"return_path" /c:"payment_compliance" *.go router\*.go controller\*.go setting\*.go service\*.go common\*.go i18n\*.go model\*.go
```

Expected: 零命中（`docs/` 除外）。`topup` 允许命中：`controller/topup.go`（AdminCompleteTopUp）、`model/user_quota.go` 日志、`router/api-router.go:97,131`、`model/log.go` LogTypeTopup、`controller/user.go` TopUp。

- [ ] **Step 6: 运行后端全量测试**

```bash
go test ./... -count=1 2>&1 | findstr /v "no test files"
```

Expected: 仅预存不稳定项失败（channel_affinity 2 个）。

- [ ] **Step 7: Commit**

```bash
git add -u
git commit --no-gpg-sign -m "refactor(payment): remove payment routes, settings, services and dependencies"
```

---

### Task 4: 前端 wallet 改造（删充值/支付，保留兑换卡片+账单历史+订阅卡片）

**Files:**
- Delete: `web/src/features/wallet/components/recharge-form-card.tsx`、`web/src/features/wallet/components/creem-products-section.tsx`、`web/src/features/wallet/components/dialogs/creem-confirm-dialog.tsx`、`web/src/features/wallet/components/dialogs/payment-confirm-dialog.tsx`
- Delete: `web/src/features/wallet/hooks/use-creem-payment.ts`、`web/src/features/wallet/hooks/use-waffo-payment.ts`、`web/src/features/wallet/hooks/use-waffo-pancake-payment.ts`、`web/src/features/wallet/hooks/use-topup-info.ts`、`web/src/features/wallet/hooks/use-payment.ts`、`web/src/features/wallet/hooks/use-payment.test.ts`
- Delete: `web/src/features/wallet/lib/payment.ts`、`web/src/features/wallet/lib/payment.test.ts`
- Create: `web/src/features/wallet/components/redemption-card.tsx`（兑换码卡片，自 recharge-form-card 提取）
- Modify: `web/src/features/wallet/index.tsx`、`web/src/features/wallet/api.ts`、`web/src/features/wallet/types.ts`、`web/src/features/wallet/components/subscription-plans-card.tsx`、`web/src/features/wallet/components/dialogs/billing-history-dialog.tsx`、`web/src/features/wallet/components/affiliate-rewards-card.tsx`、`web/src/features/wallet/hooks/use-billing-history.ts`

**Interfaces:**
- Consumes: `POST /api/user/topup`（兑换，保留）、日志 API `GET /api/user/log?type=1`（账单历史新数据源，已有 controller/log.go:36 GetUserLogs）
- Produces: `RedemptionCard` 组件（props: `redeeming: boolean; onRedeem(code: string): Promise<void>`）

- [ ] **Step 1: 删除支付相关文件**

```bash
git rm web/src/features/wallet/components/recharge-form-card.tsx web/src/features/wallet/components/creem-products-section.tsx web/src/features/wallet/components/dialogs/creem-confirm-dialog.tsx web/src/features/wallet/components/dialogs/payment-confirm-dialog.tsx web/src/features/wallet/hooks/use-creem-payment.ts web/src/features/wallet/hooks/use-waffo-payment.ts web/src/features/wallet/hooks/use-waffo-pancake-payment.ts web/src/features/wallet/hooks/use-topup-info.ts web/src/features/wallet/hooks/use-payment.ts web/src/features/wallet/hooks/use-payment.test.ts web/src/features/wallet/lib/payment.ts web/src/features/wallet/lib/payment.test.ts
```

- [ ] **Step 2: 新建 RedemptionCard 组件**

创建 `web/src/features/wallet/components/redemption-card.tsx`，从原 recharge-form-card.tsx 提取兑换码输入区块（redemption code input + Redeem 按钮 + `useRedemption()`），文案沿用现有 i18n 键（如 `Redeem`、`Redemption code`、兑换成功/失败提示键）。结构参考原文件的兑换区块，组件 props：

```tsx
interface RedemptionCardProps {
  redeeming: boolean
  onRedeem: (code: string) => Promise<void>
}
```

包含：标题（t('Redeem')）、说明文案（提示兑换码是免费获取额度的方式，i18n 键可复用现有或改英文字面量）、输入框（placeholder 复用原键）、兑换按钮（loading=redeeming）。**不包含任何金额/支付方法 UI**。

- [ ] **Step 3: 重构 wallet/index.tsx**

删除：
- `RechargeFormCard` 区块（:300-332）及其 props（topupInfo/presetAmounts/selectedPreset/topupAmount/paymentAmount/calculating/paymentLoading 等状态）
- `PaymentConfirmDialog`（:355-366）与 `CreemConfirmDialog`（:381-387）
- `AffiliateRewardsCard` 的 `complianceConfirmed` prop（:346-348）
- 相关 hooks/state（`useTopupInfo`、`usePayment`、`useCreemPayment`、`useWaffoPayment`、`useWaffoPancakePayment` 的调用与 import；`handleTopupAmountChange`、`handlePaymentMethodSelect`、`handlePaymentConfirm`、`handleCreemProductSelect`、`handleWaffoMethodSelect` 等 handler）

保留并接入：
- `<RedemptionCard redeeming={redeeming} onRedeem={handleRedeem} />` 替换原 RechargeFormCard 位置
- `<SubscriptionPlansCard userQuota={user?.quota} onPurchaseSuccess={fetchUser} />`（去掉 topupInfo props，T5 改造其内部）
- `<TransferDialog>`、`<BillingHistoryDialog>`（保留）
- `handleRedeem`/`redeemCode` state（保留）

- [ ] **Step 4: 改造 wallet/api.ts 与 use-billing-history.ts**

`web/src/features/wallet/api.ts`：
- 删除 `getTopupInfo`（:56-60 附近，调 `/api/user/topup/info`）、`getUserBillingHistory`（:217 附近，调 `/api/user/topup/self`）、`getAllBillingHistory`（:236 附近，调 `/api/user/topup` admin 列表）、`completeTopupOrder`（:246 附近，调 `/api/user/topup/complete`——**前端不再使用**，AdminCompleteTopUp 面向脚本）
- **保留** `redeemTopupCode`（:64-69，`POST /api/user/topup`）

`web/src/features/wallet/hooks/use-billing-history.ts` 改造数据源：改为调用日志接口 `GET /api/user/log?type=1`（LogTypeTopup），字段映射（id/time/content/username 等）；删掉 admin 分支（`getAllBillingHistory` 已删）。前端日志 API 用法参考 `web/src/features/usage-logs` 的现有调用（`findstr /n /c:"/api/user/log" web\src\features\usage-logs\api.ts` 确认端点）。

- [ ] **Step 5: 简化 subscription-plans-card.tsx 与 billing-history-dialog.tsx**

`subscription-plans-card.tsx`：删除 `topupInfo` prop 及其支付方法计算（enableStripe/enableCreem/enableWaffoPancake/enableOnlineTopUp/getEpayMethods 等），只保留订阅计划展示 + 余额购买按钮（调 `POST /api/subscription/balance_pay`，端点以 `findstr /n /c:"balance" router\api-router.go` 确认）。余额不足提示沿用现有 i18n。

`billing-history-dialog.tsx`：删除 admin 分支（isAdmin 相关，:76/:208/:262），适配 Step 4 的新数据源类型。

- [ ] **Step 6: 清理 wallet types.ts 与残留引用**

`types.ts` 删除 TopupInfo 等支付类型；`findstr /s /n /c:"TopupInfo" /c:"PAYMENT_TYPES" /c:"waffo" web\src\features\wallet` 逐个清理，直到零命中（保留 redemption/affiliate/billing 相关类型）。

- [ ] **Step 7: 类型检查与测试**

```bash
bun run typecheck
bun run test 2>&1 | Select-Object -Last 3
```

Expected: PASS（150 测试，若 wallet 测试被删则总数减少）。若 typecheck 报 wallet 之外文件的引用错误，`findstr /s /n /c:"wallet/payment" /c:"recharge-form-card" /c:"useTopupInfo" web\src\**\*.tsx web\src\**\*.ts` 定位清理。

- [ ] **Step 8: Commit**

```bash
git add -u
git commit --no-gpg-sign -m "refactor(payment): rebuild wallet page with redemption card and log-based history"
```

---

### Task 5: 前端订阅/设置页/导航清理（订阅购买简化、支付设置 section 删除、充值入口改名）

**Files:**
- Modify: `web/src/features/subscriptions/components/dialogs/subscription-purchase-dialog.tsx`
- Modify: `web/src/features/system-settings/billing/index.tsx`、`web/src/features/system-settings/billing/section-registry.tsx`
- Delete: `web/src/features/system-settings/integrations/payment-settings-section.tsx`（确认路径后）
- Modify: `web/src/features/system-settings/types.ts`、`web/src/features/system-settings/utils/section-registry.ts`
- Modify: `web/src/features/profile/components/sidebar-modules-card.tsx:109`

**Interfaces:**
- Consumes: 保留的订阅 API（plans/self/purchase 相关，路由未删）
- Produces: 无新接口

- [ ] **Step 1: 简化订阅购买弹窗**

`subscription-purchase-dialog.tsx`：删除支付方法选择（epayMethods/stripe/creem/waffo 选项渲染与 `enableOnlineTopUp`/`enableStripe` 等 props），弹窗仅保留：计划信息 + 余额（PriceAmount）展示 + 「余额购买」按钮 + 余额不足提示（复用现有键如 `Insufficient balance`）。购买调用保留 `POST /api/subscription/balance_pay`（端点以 `findstr /n /c:"balance_pay" /c:"RequestBalancePay" router\api-router.go controller\subscription.go` 确认）。

- [ ] **Step 2: 删除支付设置 section**

`web/src/features/system-settings/billing/section-registry.tsx`：删除 payment section（引用 `PaymentSettingsSection` 处，:24 import + section 定义）；`web/src/features/system-settings/billing/index.tsx`：`defaultBillingSettings` 删除支付键（TopUpLink、PayAddress、EpayId、EpayKey、Price、MinTopUp、CustomCallbackAddress、PayMethods、payment_setting.*、Stripe*、Creem*、Waffo* 等，:32/:62-105 区间）；`quota` section 构建中删除 TopUpLink（:66）与 complianceConfirmed（:75-78）；`QuotaSettingsSection` 组件中 TopUpLink/compliance 相关字段删除（在 `web/src/features/system-settings/general/quota-settings-section.tsx`）。

删除支付 section 文件（确认实际路径：`Get-ChildItem web\src\features\system-settings\integrations -Name`）。

`web/src/features/system-settings/types.ts`：删除 `BillingSettings` 中的支付字段。`utils/section-registry.ts`：若含支付 section id 硬编码则清理。

- [ ] **Step 3: 侧边栏入口改名**

`web/src/features/profile/components/sidebar-modules-card.tsx:109`：`key: 'topup'` 的模块项 title 改为 `t('Exchange')`（或复用现有 `Redeem` 键），指向钱包页 `/wallet`。若该文件用模块 key 映射路由，同步更新（`findstr /n /c:"topup" web\src\features\profile\components\sidebar-modules-card.tsx` 确认完整上下文）。

- [ ] **Step 4: 前端残留扫描与类型检查**

```bash
bun run typecheck
findstr /s /n /c:"TopUpLink" /c:"StripeMinTopUp" /c:"payment_setting" /c:"enable_stripe_topup" /c:"PAYMENT_TYPES" /c:"waffo" web\src\**\*.tsx web\src\**\*.ts 2>$null | Where-Object { $_ -notmatch "locales" }
```

Expected: typecheck PASS；扫描零命中（locales 除外）。

- [ ] **Step 5: Commit**

```bash
git add -u
git commit --no-gpg-sign -m "refactor(payment): simplify subscription purchase, remove payment settings section, rename nav entry"
```

---

### Task 6: i18n 死键清理 + 全量验证 + 单次提交

**Files:**
- Modify: `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`
- Modify: `go.mod`、`go.sum`（若 T3 有遗漏）

**Interfaces:**
- Consumes: 前 5 个任务的全部产出
- Produces: 最终单次提交

- [ ] **Step 1: 收集死 i18n 键**

用临时脚本扫描：遍历 7 个 locale 文件的所有键，排除在 `web/src`（locales/static-keys 目录除外）源码中被 `t('...')`、`i18n` 引用、以及 `static-keys.ts` 登记的键。支付相关死键（参考键名：Stripe/creem/waffo/epay/payment/topup/recharge/online payment/compliance 相关文案）列入删除清单。注意**保留**兑换/订阅/邀请/签到相关键。

- [ ] **Step 2: 删除死键**

参照上一轮方式：写临时脚本 `web/remove-keys.tmp.cjs`（读 `src/i18n/locales/{lang}.json` 的 `data.translation` 对象删除命中的键），运行后删除脚本：

```js
const fs = require('fs')
const keys = [/* Step 1 收集的键 */]
for (const lang of ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi']) {
  const p = `src/i18n/locales/${lang}.json`
  const data = JSON.parse(fs.readFileSync(p, 'utf8'))
  let n = 0
  for (const k of keys) {
    if (k in data.translation) { delete data.translation[k]; n++ }
  }
  fs.writeFileSync(p, JSON.stringify(data, null, 2) + '\n')
  console.log(`${lang}: removed ${n}`)
}
```

- [ ] **Step 3: i18n 同步与前端验证**

```bash
bun run i18n:sync
bun run typecheck
bun run lint
bun run test
bun run build
```

Expected: 全部 PASS（`bun run lint` 若有全局存量 warning 不阻塞，但本次涉及文件不得有 error）。

- [ ] **Step 4: 后端全量验证与残留扫描**

```bash
go build ./... && go vet ./...
go test ./... -count=1 2>&1 | findstr /v "no test files"
findstr /s /m /c:"stripe" /c:"creem" /c:"waffo" /c:"epay" /c:"return_path" /c:"payment_compliance" /c:"TopUpLink" /c:"TopUpStatus" controller\*.go model\*.go setting\*.go service\*.go common\*.go router\*.go i18n\*.go
```

Expected: build/vet/test PASS（仅预存 channel_affinity 失败）；扫描零命中。`topup` 关键词允许命中（兑换码兑换 + AdminCompleteTopUp + LogTypeTopup）。

- [ ] **Step 5: 前端残留扫描**

```bash
findstr /s /m /c:"recharge-form-card" /c:"useTopupInfo" /c:"payment-confirm-dialog" /c:"creem" /c:"waffo" /c:"stripe" /c:"epay" web\src 2>$null | Where-Object { $_ -notmatch "locales" }
```

Expected: 零命中（locales 除外；模型名/分类中可能含 doubao 类关键词不算）。

- [ ] **Step 6: 整理提交历史为单次提交**

```bash
git log --oneline -8
```

若出现多个 `refactor(payment):` 提交，将其合并（`git reset --soft` 到本计划首个提交之前，重新提交一次）。提交信息：

```bash
git add -u
git commit --no-gpg-sign -m "refactor(payment): remove payment system, make public free relay (redeem/manual credit/subscription kept)"
```

注意：`git add -u` 只暂存跟踪文件改动；若 `git rm` 已执行则无需额外处理。确认 `git status --short` 无 `.codegraph/`、`docs/learn/`、`opencode.json` 混入。

- [ ] **Step 7: 最终确认**

```bash
git log --oneline -3
git status --short
```

Expected: 单次提交（+spec 文档提交可保留为独立 docs 提交），工作区干净（untracked 的 .codegraph/ 等除外）。

---

## 自审（Self-Review）

**Spec 覆盖检查：**
- 在线充值全套删除 → T2/T3（控制器、路由、依赖）
- 订阅支付回调删除 → T2
- 支付合规/回跳删除 → T2/T3（payment_compliance.go、return_path.go、i18n 键）
- 充值设置 section 删除 → T5（integrations/payment-settings-section）
- 充值记录页删除 → T2（GetAllTopUps）+ T4（billing-history 改造）
- 订阅保留 + 余额购买 → T1（PurchaseSubscriptionWithBalance 改造）+ T4/T5
- AdminCompleteTopUp 改造 → T2（userId+quota）+ T3（路由保留）
- 兑换码保留 → T2（user.go TopUp 去合规）+ 全计划不触碰 redemption.go
- 迁移删除表（不写 DROP）→ T1（AutoMigrate 列表移除）
- 计费设置页保留 → T5 明确只删支付 section
- 导航改名 → T5
- 前端 i18n 清理 → T6

**占位符检查：** T1 Step 4-5 已明确日志行写法；各任务命令含具体文件/行号/端点确认方式。T5 Step 1/2 中「确认端点/路径」步骤给出了 findstr 定位命令，无 TBD。

**类型一致性：** `ManualCreditQuota(userId, quota int) error` 在 T1 定义、T2 消费；`RedemptionCard` props 在 T4 定义并消费；`AdminManualTopupRequest{UserId, Quota}` 与 T3 路由保留一致。

**已知风险：** `service/return_path.go` 的 `PaymentReturnURL` 与 `controller/return_path.go` 的 `paymentReturnPath` 是两个不同文件（controller 层已被 T2 删；service 层 T3 删，需零引用确认）；`setting/operation_setting` 中支付定义分布需 T3 Step 2 按 findstr 结果逐一确认（已知 payment_setting.go 必删，MinTopUp/Price 等若定义在 general_setting.go 则保留 QuotaDisplayType 部分）。
