# 代理质量检测报告弹窗 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 质量检测（Quality）返回逐 AI 目标明细并弹报告弹窗，Test/Quality 按钮带 loading 与失败 toast。

**Architecture:** 后端 `service.ProxyProbeResult` 增加 `Items []model.ProxyQualityItem`（base_connectivity + 4 个 AI 目标），controller 仅在质量检测响应中透出（`omitempty`，不落库）；前端新增 `QualityReportDialog`（复用 `@/components/dialog` 高层封装与 `StatusBadge`），`Proxies` 页新增 running 状态与 toast。

**Tech Stack:** Go/Gin/GORM、React 19、TanStack Query、i18next、Base UI、Tailwind、Vitest、Bun。

## Global Constraints

- 设计文档与实施计划**不提交 git**（用户要求）；代码与测试照常提交。
- 后端：JSON 一律走 `common.Marshal`/`common.Unmarshal`；测试用 `require`（setup/fatal）+ `assert`（非 fatal）；质量检测仍不修改代理状态、不落库明细。
- 前端：测试文件必须放模块专属 `__tests__/` 目录；组件 props 不解构，用 `props.xxx`；新增功能必须带测试；改动后必须 `bun run typecheck` + lint 涉及文件。
- i18n：en 为源键，zh/zh-TW/fr/ru/ja/vi 全量；先查重，只加缺失键。
- 质量检测请求最长 10 秒（controller 上下文），loading 在请求期间持续。

---

### Task 1: 后端探测明细 items（model + service + controller）

**Files:**
- Modify: `model/proxy.go`（ProxySummary 后新增 ProxyQualityItem 类型 + QualityItems 字段）
- Modify: `service/proxy_probe.go`
- Modify: `controller/proxy.go:319-325`（runProxyProbe 尾部）
- Test: `controller/proxy_probe_test.go`

**Interfaces:**
- Consumes: 现有 `service.ProxyProbe` 接口、`proxySummary(proxy)`、`fixedProxyProbe` 测试桩。
- Produces:
  - `model.ProxyQualityItem{ Target, URL, Status, HTTPStatus, LatencyMS, Message, CFRay string/int }`，json 标签 `target / url,omitempty / status / http_status,omitempty / latency_ms,omitempty / message,omitempty / cf_ray,omitempty`
  - `model.ProxySummary.QualityItems []ProxyQualityItem json:"quality_items,omitempty"`
  - `service.ProxyProbeResult.Items []model.ProxyQualityItem`
  - `controller.runProxyProbe`：`quality == true` 时 `summary.QualityItems = result.Items`，Test 响应不带

- [ ] **Step 1: 写失败测试**

在 `controller/proxy_probe_test.go` 追加两个测试：

```go
func TestProxyQualityCheckReturnsPerTargetItems(t *testing.T) {
	setupProxyControllerTestDB(t)
	proxy := model.Proxy{Name: "quality-items", Protocol: model.ProxyProtocolHTTP, Host: "example.com", Port: 8080, Status: model.ProxyStatusInactive}
	require.NoError(t, proxy.Insert())
	previous := proxyProbe
	proxyProbe = fixedProxyProbe{result: service.ProxyProbeResult{
		LatencyMS:     123,
		HTTPStatus:    200,
		IPAddress:     "203.0.113.8",
		Country:       "Testland",
		QualityStatus: "warn",
		QualityScore:  70,
		QualityGrade:  "B",
		Summary:       "通过 1 项，告警 1 项，失败 2 项，挑战 1 项",
		Items: []model.ProxyQualityItem{
			{Target: "base_connectivity", Status: "pass", HTTPStatus: 200, LatencyMS: 123, Message: "代理出口连通正常"},
			{Target: "openai", URL: "https://api.openai.com/v1/models", Status: "pass", HTTPStatus: 401, LatencyMS: 300, Message: "目标可达"},
			{Target: "anthropic", URL: "https://api.anthropic.com/v1/messages", Status: "challenge", HTTPStatus: 403, LatencyMS: 500, Message: "目标返回 Cloudflare 挑战", CFRay: "abc123"},
			{Target: "gemini", URL: "https://generativelanguage.googleapis.com/$discovery/rest?version=v1beta", Status: "fail", LatencyMS: 1000, Message: "探测请求失败: context deadline exceeded"},
			{Target: "grok", URL: "https://api.x.ai/v1/models", Status: "warn", HTTPStatus: 429, LatencyMS: 800, Message: "目标被限流"},
		},
	}}
	t.Cleanup(func() { proxyProbe = previous })

	router := gin.New()
	router.POST("/proxy/:id/quality-check", CheckProxyQuality)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/proxy/%d/quality-check", proxy.ID), nil))

	body := recorder.Body.String()
	assert.Contains(t, body, `"quality_items":[`)
	assert.Contains(t, body, `"target":"base_connectivity"`)
	assert.Contains(t, body, `"url":"https://api.openai.com/v1/models"`)
	assert.Contains(t, body, `"status":"challenge"`)
	assert.Contains(t, body, `"cf_ray":"abc123"`)
	assert.Contains(t, body, `"http_status":429`)
	stored, err := model.GetProxyByID(proxy.ID)
	require.NoError(t, err)
	assert.Equal(t, model.ProxyStatusInactive, stored.Status)
	assert.Equal(t, 70, *stored.QualityScore)
}

func TestProxyTestResponseOmitsQualityItems(t *testing.T) {
	setupProxyControllerTestDB(t)
	proxy := model.Proxy{Name: "test-omit", Protocol: model.ProxyProtocolHTTP, Host: "example.com", Port: 8080}
	require.NoError(t, proxy.Insert())
	previous := proxyProbe
	proxyProbe = fixedProxyProbe{result: service.ProxyProbeResult{
		LatencyMS:     50,
		HTTPStatus:    200,
		IPAddress:     "203.0.113.9",
		QualityStatus: "healthy",
		QualityScore:  100,
		QualityGrade:  "A",
		Summary:       "ok",
		Items: []model.ProxyQualityItem{
			{Target: "base_connectivity", Status: "pass", HTTPStatus: 200, LatencyMS: 50, Message: "代理出口连通正常"},
		},
	}}
	t.Cleanup(func() { proxyProbe = previous })

	router := gin.New()
	router.POST("/proxy/:id/test", TestProxy)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/proxy/%d/test", proxy.ID), nil))

	assert.NotContains(t, recorder.Body.String(), "quality_items")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./controller/ -run "TestProxyQualityCheckReturnsPerTargetItems|TestProxyTestResponseOmitsQualityItems" -count=1 -v`
Expected: FAIL — `service.ProxyProbeResult` 无 `Items` 字段（编译错误），或响应无 `quality_items`。

- [ ] **Step 3: model/proxy.go 增加类型**

在 `model/proxy.go` 的 `ProxySummary` struct 之后追加：

```go
// ProxyQualityItem is one AI target's latest quality check result, returned
// only in the quality-check API response (never persisted).
type ProxyQualityItem struct {
	Target     string `json:"target"`
	URL        string `json:"url,omitempty"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	LatencyMS  int    `json:"latency_ms,omitempty"`
	Message    string `json:"message,omitempty"`
	CFRay      string `json:"cf_ray,omitempty"`
}
```

在 `ProxySummary` struct 末尾（`BoundChannelCount int64 json:"bound_channel_count"` 之后）追加字段：

```go
	QualityItems []ProxyQualityItem `json:"quality_items,omitempty"`
```

- [ ] **Step 4: service/proxy_probe.go 产出 items**

修改 `service/proxy_probe.go`：

1. import 增加 `"github.com/QuantumNous/new-api/model"`。
2. `ProxyProbeResult` 末尾增加 `Items []model.ProxyQualityItem`。
3. 新增辅助函数：

```go
func baseConnectivityItem(status string, httpStatus, latencyMS int, message string) model.ProxyQualityItem {
	return model.ProxyQualityItem{Target: "base_connectivity", Status: status, HTTPStatus: httpStatus, LatencyMS: latencyMS, Message: message}
}
```

4. `probeExitInfo`：两处成功 `return last` 之前各插入一行；末尾失败路径（`last.QualityStatus, ... = "failed", 78, "B"` 之后、`return last` 之前）插入一行：

```go
		last.Items = append(last.Items, baseConnectivityItem("pass", last.HTTPStatus, last.LatencyMS, "代理出口连通正常"))
```

```go
	last.Items = append(last.Items, baseConnectivityItem("fail", last.HTTPStatus, last.LatencyMS, last.Error))
```

5. `probeQualityTarget` 整体替换为返回 `model.ProxyQualityItem`（保留现有分类语义，新增延迟/消息/cf_ray）：

```go
func probeQualityTarget(ctx context.Context, client *http.Client, target proxyQualityTarget) model.ProxyQualityItem {
	item := model.ProxyQualityItem{Target: target.name, URL: target.url}
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.url, nil)
	if err != nil {
		item.Status = "fail"
		item.Message = "创建探测请求失败"
		return item
	}
	req.Header.Set("Accept", "application/json,text/html,*/*")
	req.Header.Set("User-Agent", "new-api-proxy-quality/1.0")
	resp, err := client.Do(req)
	item.LatencyMS = int(time.Since(started).Milliseconds())
	if err != nil {
		item.Status = "fail"
		item.Message = fmt.Sprintf("探测请求失败: %v", err)
		return item
	}
	item.HTTPStatus = resp.StatusCode
	item.CFRay = resp.Header.Get("CF-Ray")
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	_ = resp.Body.Close()
	lowerBody := strings.ToLower(string(body))
	if resp.StatusCode == http.StatusForbidden && (item.CFRay != "" || strings.Contains(lowerBody, "cloudflare") || strings.Contains(lowerBody, "challenge")) {
		item.Status = "challenge"
		item.Message = "目标返回 Cloudflare 挑战"
		return item
	}
	if _, ok := target.allowed[resp.StatusCode]; ok {
		item.Status = "pass"
		item.Message = "目标可达"
		return item
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		item.Status = "pass"
		item.Message = "目标可达"
		return item
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		item.Status = "warn"
		item.Message = "目标被限流"
		return item
	}
	item.Status = "fail"
	item.Message = fmt.Sprintf("目标不可达 (HTTP %d)", resp.StatusCode)
	return item
}
```

6. `Probe` 的质量循环改为收集 items：

```go
	passed, warned, failed, challenged := 1, 0, 0, 0
	for _, target := range defaultProxyQualityTargets {
		item := probeQualityTarget(ctx, client, target)
		result.Items = append(result.Items, item)
		switch item.Status {
		case "pass":
			passed++
		case "warn":
			warned++
		case "challenge":
			challenged++
		default:
			failed++
		}
	}
```

（原 `status := probeQualityTarget(...)` + `switch status` 整体替换。）

- [ ] **Step 5: controller/proxy.go 透出 items**

将 `runProxyProbe` 末尾：

```go
	recordManageAudit(c, "proxy.quality_check", map[string]interface{}{"id": id, "quality": quality})
	common.ApiSuccess(c, proxySummary(proxy))
```

替换为：

```go
	recordManageAudit(c, "proxy.quality_check", map[string]interface{}{"id": id, "quality": quality})
	summary := proxySummary(proxy)
	if quality {
		summary.QualityItems = result.Items
	}
	common.ApiSuccess(c, summary)
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./controller/ -run "TestProxy" -count=1 -v`
Expected: 三个 Proxy 测试（含原有 `TestProxyQualityCheckPersistsLatestWithoutChangingStatus`）全部 PASS。

- [ ] **Step 7: 提交**

```bash
git add model/proxy.go service/proxy_probe.go controller/proxy.go controller/proxy_probe_test.go
git commit -m "feat(proxy): return per-target quality check items in quality-check API"
```

---

### Task 2: 前端 api 类型 + QualityReportDialog 组件 + en/zh i18n 键

**Files:**
- Modify: `web/src/features/proxies/api.ts`
- Create: `web/src/features/proxies/components/quality-report-dialog.tsx`
- Test: `web/src/features/proxies/components/__tests__/quality-report-dialog.test.tsx`
- Modify: `web/src/i18n/locales/en.json`、`web/src/i18n/locales/zh.json`（只加缺失键，先查重）

**Interfaces:**
- Consumes: Task 1 的 `quality_items` 响应字段（json: target/url/status/http_status/latency_ms/message/cf_ray）。
- Produces:
  - `ProxyQualityItem` 类型 + `ProxySummary.quality_items?: ProxyQualityItem[]`
  - `QualityReportDialog` 组件，props: `{ proxy: ProxySummary | null; open: boolean; onOpenChange: (open: boolean) => void }`

- [ ] **Step 1: api.ts 增加类型**

在 `web/src/features/proxies/api.ts` 的 `ProxySummary` 接口之前增加：

```typescript
export interface ProxyQualityItem {
  target: string
  url?: string
  status: 'pass' | 'warn' | 'challenge' | 'fail' | string
  http_status?: number
  latency_ms?: number
  message?: string
  cf_ray?: string
}
```

`ProxySummary` 接口末尾（`bound_channel_count: number` 之后）增加：

```typescript
  quality_items?: ProxyQualityItem[]
```

- [ ] **Step 2: 写失败组件测试**

创建 `web/src/features/proxies/components/__tests__/quality-report-dialog.test.tsx`：

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import type { ProxySummary } from '../../api'

const { QualityReportDialog } = await import('../quality-report-dialog')

const baseProxy: ProxySummary = {
  id: 1,
  name: 'test-proxy',
  protocol: 'http',
  host: 'example.com',
  port: 8080,
  status: 'active',
  credential_configured: false,
  credential_decrypt_failed: false,
  bound_channel_count: 2,
  quality_status: 'warn',
  quality_score: 70,
  quality_grade: 'B',
  quality_summary: '通过 1 项，告警 1 项，失败 2 项，挑战 1 项',
  ip_address: '203.0.113.8',
  country: 'Testland',
  latency_ms: 123,
  last_checked_at: 1787829618,
  quality_items: [
    { target: 'base_connectivity', status: 'pass', http_status: 200, latency_ms: 123, message: '代理出口连通正常' },
    { target: 'openai', url: 'https://api.openai.com/v1/models', status: 'pass', http_status: 401, latency_ms: 300, message: '目标可达' },
    { target: 'anthropic', url: 'https://api.anthropic.com/v1/messages', status: 'challenge', http_status: 403, latency_ms: 500, message: '目标返回 Cloudflare 挑战', cf_ray: 'abc123' },
    { target: 'gemini', url: 'https://generativelanguage.googleapis.com/$discovery/rest?version=v1beta', status: 'fail', latency_ms: 1000, message: '探测请求失败: context deadline exceeded' },
    { target: 'grok', url: 'https://api.x.ai/v1/models', status: 'warn', http_status: 429, latency_ms: 800, message: '目标被限流' },
  ],
}

describe('QualityReportDialog', () => {
  test('renders summary and per-target rows with brand name, url and status', () => {
    render(
      <QualityReportDialog proxy={baseProxy} open onOpenChange={() => undefined} />
    )

    expect(screen.getByText('Quality Report')).toBeInTheDocument()
    expect(screen.getByText('70')).toBeInTheDocument()
    expect(screen.getByText(/Grade: B/)).toBeInTheDocument()
    expect(screen.getByText('通过 1 项，告警 1 项，失败 2 项，挑战 1 项')).toBeInTheDocument()
    expect(screen.getByText(/203\.0\.113\.8/)).toBeInTheDocument()
    expect(screen.getByText(/Testland/)).toBeInTheDocument()
    expect(screen.getByText(/123ms/)).toBeInTheDocument()

    expect(screen.getByText('Base connectivity')).toBeInTheDocument()
    expect(screen.getByText('OpenAI')).toBeInTheDocument()
    expect(screen.getByText('https://api.openai.com/v1/models')).toBeInTheDocument()
    expect(screen.getByText('Anthropic')).toBeInTheDocument()
    expect(screen.getByText('Gemini')).toBeInTheDocument()
    expect(screen.getByText('Grok')).toBeInTheDocument()

    expect(screen.getAllByText('Pass')).toHaveLength(2)
    expect(screen.getByText('Warn')).toBeInTheDocument()
    expect(screen.getByText('Challenge')).toBeInTheDocument()
    expect(screen.getByText('Fail')).toBeInTheDocument()

    expect(screen.getByText('429')).toBeInTheDocument()
    expect(screen.getByText('800ms')).toBeInTheDocument()
    expect(screen.getByText(/cf-ray: abc123/)).toBeInTheDocument()
    expect(screen.getByText('探测请求失败: context deadline exceeded')).toBeInTheDocument()
  })

  test('renders dash fallbacks for missing fields and empty-state without items', () => {
    const emptyProxy: ProxySummary = {
      ...baseProxy,
      ip_address: undefined,
      country: undefined,
      latency_ms: null,
      last_checked_at: null,
      quality_items: [],
    }
    render(
      <QualityReportDialog proxy={emptyProxy} open onOpenChange={() => undefined} />
    )

    expect(screen.getByText('No quality results')).toBeInTheDocument()
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(3)
  })

  test('renders nothing when proxy is null', () => {
    render(<QualityReportDialog proxy={null} open onOpenChange={() => undefined} />)
    expect(screen.queryByText('Quality Report')).not.toBeInTheDocument()
  })
})
```

- [ ] **Step 3: 运行测试确认失败**

Run（在 `web/` 目录）: `bunx vitest run src/features/proxies/components/__tests__/quality-report-dialog.test.tsx`
Expected: FAIL — 模块 `../quality-report-dialog` 不存在。

- [ ] **Step 4: 实现组件**

创建 `web/src/features/proxies/components/quality-report-dialog.tsx`：

```tsx
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { formatTimestamp } from '@/lib/format'

import type { ProxyQualityItem, ProxySummary } from '../api'

const targetLabelKeys: Record<string, string> = {
  base_connectivity: 'Base connectivity',
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
  grok: 'Grok',
}

const statusVariantMap: Record<string, 'success' | 'warning' | 'danger'> = {
  pass: 'success',
  warn: 'warning',
  challenge: 'danger',
  fail: 'danger',
}

const statusLabelKeys: Record<string, string> = {
  pass: 'Pass',
  warn: 'Warn',
  challenge: 'Challenge',
  fail: 'Fail',
}

export function QualityReportDialog(props: {
  proxy: ProxySummary | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  if (!props.proxy) {
    return null
  }
  const proxy = props.proxy
  const items = proxy.quality_items ?? []
  const targetLabel = (item: ProxyQualityItem) =>
    t(targetLabelKeys[item.target] ?? item.target)
  const statusLabel = (item: ProxyQualityItem) =>
    t(statusLabelKeys[item.status] ?? item.status)
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Quality Report')}
      footer={
        <Button variant='outline' onClick={() => props.onOpenChange(false)}>
          {t('Close')}
        </Button>
      }
    >
      <div className='bg-muted/40 rounded-lg border p-4'>
        <div className='flex items-center justify-between gap-4'>
          <div className='min-w-0'>
            <div className='text-muted-foreground text-sm'>{proxy.name}</div>
            <div className='mt-1 text-sm'>{proxy.quality_summary || '-'}</div>
          </div>
          <div className='text-right'>
            <div className='text-2xl font-semibold'>{proxy.quality_score ?? '-'}</div>
            <div className='text-muted-foreground text-xs'>
              {t('Grade')}: {proxy.quality_grade || '-'}
            </div>
          </div>
        </div>
        <div className='text-muted-foreground mt-3 grid grid-cols-2 gap-2 text-xs'>
          <div>
            {t('Exit IP')}: {proxy.ip_address || '-'}
          </div>
          <div>
            {t('Country')}: {proxy.country || '-'}
          </div>
          <div>
            {t('Latency')}: {proxy.latency_ms != null ? `${proxy.latency_ms}ms` : '-'}
          </div>
          <div>
            {t('Checked at')}:{' '}
            {proxy.last_checked_at != null ? formatTimestamp(proxy.last_checked_at) : '-'}
          </div>
        </div>
      </div>
      <div className='overflow-x-auto rounded-lg border'>
        <table className='w-full text-sm'>
          <thead>
            <tr className='border-b text-left'>
              <th className='p-3'>{t('Target')}</th>
              <th className='p-3'>{t('Status')}</th>
              <th className='p-3'>HTTP</th>
              <th className='p-3'>{t('Latency')}</th>
              <th className='p-3'>{t('Message')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.target} className='border-b last:border-0'>
                <td className='p-3'>
                  <div className='font-medium'>{targetLabel(item)}</div>
                  {item.url ? (
                    <div className='text-muted-foreground break-all text-xs'>
                      {item.url}
                    </div>
                  ) : null}
                </td>
                <td className='p-3'>
                  <StatusBadge
                    variant={statusVariantMap[item.status] ?? 'neutral'}
                    copyable={false}
                  >
                    {statusLabel(item)}
                  </StatusBadge>
                </td>
                <td className='p-3'>{item.http_status ?? '-'}</td>
                <td className='p-3'>
                  {item.latency_ms != null ? `${item.latency_ms}ms` : '-'}
                </td>
                <td className='p-3'>
                  <span>{item.message || '-'}</span>
                  {item.cf_ray ? (
                    <span className='text-muted-foreground ml-1 text-xs'>
                      (cf-ray: {item.cf_ray})
                    </span>
                  ) : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {items.length === 0 && (
          <div className='text-muted-foreground p-8 text-center'>
            {t('No quality results')}
          </div>
        )}
      </div>
    </Dialog>
  )
}
```

注意：`t('Base connectivity')` 这类键的翻译值在 en.json 中就是键本身（"Base connectivity"），zh.json 中是中文（"基础连通性"）；`targetLabelKeys`/`statusLabelKeys` 中 OpenAI/Anthropic/Gemini/Grok 为品牌名，不翻译（en.json 值=键）。

- [ ] **Step 5: 运行测试确认通过**

Run（在 `web/` 目录）: `bunx vitest run src/features/proxies/components/__tests__/quality-report-dialog.test.tsx`
Expected: PASS（3 个用例）。

- [ ] **Step 6: 补充 en/zh 翻译键**

先查重（例如 `findstr "Quality Report" web/src/i18n/locales/en.json`），只追加缺失键。键列表：

`Quality Report`、`Score`、`Grade`、`Exit IP`、`Country`、`Latency`、`Checked at`、`Target`、`Status`、`Message`、`Base connectivity`、`Pass`、`Warn`、`Challenge`、`Fail`、`No quality results`

en.json：值=键（如 `"Quality Report": "Quality Report"`）。
zh.json：`质量检测报告`、`分数`、`等级`、`出口 IP`、`国家`、`延迟`、`检测时间`、`目标`、`状态`、`消息`、`基础连通性`、`通过`、`警告`、`挑战`、`失败`、`暂无质量结果`。

- [ ] **Step 7: 提交**

```bash
git add web/src/features/proxies/api.ts web/src/features/proxies/components/quality-report-dialog.tsx web/src/features/proxies/components/__tests__/quality-report-dialog.test.tsx web/src/i18n/locales/en.json web/src/i18n/locales/zh.json
git commit -m "feat(web): add proxy quality report dialog with per-target ratings"
```

---

### Task 3: Proxies 页交互（loading / toast / 弹窗联动）

**Files:**
- Modify: `web/src/features/proxies/index.tsx`
- Test: `web/src/features/proxies/__tests__/proxies-page.test.tsx`（新建目录 `__tests__`）

**Interfaces:**
- Consumes: Task 2 的 `QualityReportDialog`、`ProxySummary`（含 `quality_items`）。
- Produces: `Proxies` 页 Test/Quality 按钮的 loading 态、失败 toast、Quality 成功打开报告弹窗。

- [ ] **Step 1: 写失败页面测试**

创建 `web/src/features/proxies/__tests__/proxies-page.test.tsx`：

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

import { toast } from 'sonner'

const { api } = await import('@/lib/api')
const { Proxies } = await import('../index')

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = { get: ApiMethod; post: ApiMethod }

const apiClient = api as unknown as MockableApi

const proxySummary = {
  id: 1,
  name: 'p1',
  protocol: 'http',
  host: 'example.com',
  port: 8080,
  status: 'active',
  credential_configured: false,
  credential_decrypt_failed: false,
  bound_channel_count: 0,
}

const summaryWithItems = {
  ...proxySummary,
  quality_status: 'warn',
  quality_score: 70,
  quality_grade: 'B',
  quality_summary: '通过 1 项，告警 1 项，失败 2 项，挑战 1 项',
  ip_address: '203.0.113.8',
  latency_ms: 123,
  last_checked_at: 1787829618,
  quality_items: [
    {
      target: 'openai',
      url: 'https://api.openai.com/v1/models',
      status: 'pass',
      http_status: 401,
      latency_ms: 300,
      message: '目标可达',
    },
  ],
}

function installApiFixtures(options?: { qualityMessage?: string; qualitySuccess?: boolean }) {
  const calls: string[] = []
  apiClient.get = async (url) => {
    calls.push(`GET ${url}`)
    if (url === '/api/proxy/') {
      return { data: { success: true, data: { items: [proxySummary], total: 1, page: 1, page_size: 100 } } }
    }
    throw new Error(`Unexpected GET ${url}`)
  }
  apiClient.post = async (url) => {
    calls.push(`POST ${url}`)
    if (url === '/api/proxy/1/quality-check') {
      return {
        data:
          options?.qualitySuccess === false
            ? { success: false, message: options.qualityMessage }
            : { success: true, data: summaryWithItems },
      }
    }
    if (url === '/api/proxy/1/test') {
      return { data: { success: true, data: proxySummary } }
    }
    throw new Error(`Unexpected POST ${url}`)
  }
  return { calls: () => calls }
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <Proxies />
    </QueryClientProvider>
  )
}

describe('Proxies page quality interactions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  test('opens quality report dialog with per-target items after clicking Quality', async () => {
    const { calls } = installApiFixtures()
    renderPage()
    const user = userEvent.setup()

    expect(await screen.findByText('p1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Quality' }))

    expect(await screen.findByText('Quality Report')).toBeInTheDocument()
    expect(screen.getByText('OpenAI')).toBeInTheDocument()
    expect(screen.getByText('https://api.openai.com/v1/models')).toBeInTheDocument()
    expect(calls()).toContain('POST /api/proxy/1/quality-check')
    expect(toast.error).not.toHaveBeenCalled()
  })

  test('runs test without opening dialog and without error toast on success', async () => {
    const { calls } = installApiFixtures()
    renderPage()
    const user = userEvent.setup()

    expect(await screen.findByText('p1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Test' }))

    await waitFor(() => {
      expect(calls()).toContain('POST /api/proxy/1/test')
    })
    expect(screen.queryByText('Quality Report')).not.toBeInTheDocument()
    expect(toast.error).not.toHaveBeenCalled()
  })

  test('shows error toast and no dialog when quality check fails', async () => {
    installApiFixtures({ qualitySuccess: false, qualityMessage: '探测失败' })
    renderPage()
    const user = userEvent.setup()

    expect(await screen.findByText('p1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Quality' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('探测失败')
    })
    expect(screen.queryByText('Quality Report')).not.toBeInTheDocument()
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run（在 `web/` 目录）: `bunx vitest run src/features/proxies/__tests__/proxies-page.test.tsx`
Expected: FAIL — 页面无 Quality 报告交互（弹窗不出现 / toast 未调用）。

- [ ] **Step 3: 实现页面交互**

修改 `web/src/features/proxies/index.tsx`：

1. imports 增加：

```tsx
import { toast } from 'sonner'

import { Spinner } from '@/components/ui/spinner'
```

并增加 `import { QualityReportDialog } from './components/quality-report-dialog'`（放现有 `ProxyMutateDrawer` import 之后）。

2. `Proxies` 组件内新增 state（`drawerOpen` 声明之后）：

```tsx
  const [running, setRunning] = useState<{
    id: number
    kind: 'test' | 'quality'
  } | null>(null)
  const [reportProxy, setReportProxy] = useState<ProxySummary | null>(null)
```

3. 新增处理函数（现有 `action` 之后）：

```tsx
  const runProxyAction = async (
    proxy: ProxySummary,
    kind: 'test' | 'quality',
    fn: (id: number) => Promise<{
      success: boolean
      message?: string
      data?: ProxySummary
    }>,
    errorMessage: string
  ) => {
    setRunning({ id: proxy.id, kind })
    try {
      const response = await fn(proxy.id)
      if (!response.success) {
        throw new Error(response.message || errorMessage)
      }
      return response
    } catch (error) {
      toast.error(error instanceof Error ? error.message : errorMessage)
      return null
    } finally {
      setRunning(null)
    }
  }
  const handleTest = async (proxy: ProxySummary) => {
    const response = await runProxyAction(proxy, 'test', testProxy, t('Proxy test failed'))
    if (response) {
      refresh()
    }
  }
  const handleQualityCheck = async (proxy: ProxySummary) => {
    const response = await runProxyAction(
      proxy,
      'quality',
      qualityCheckProxy,
      t('Quality check failed')
    )
    if (!response) {
      return
    }
    if (response.data) {
      setReportProxy(response.data)
    }
    refresh()
  }
```

4. Test/Quality 按钮替换为带 loading 与新 handler：

```tsx
                        <Button
                          size='sm'
                          variant='ghost'
                          disabled={running?.id === proxy.id}
                          onClick={() => void handleTest(proxy)}
                        >
                          {running?.id === proxy.id && running.kind === 'test' ? (
                            <Spinner className='mr-1' />
                          ) : null}
                          {t('Test')}
                        </Button>
                        <Button
                          size='sm'
                          variant='ghost'
                          disabled={running?.id === proxy.id}
                          onClick={() => void handleQualityCheck(proxy)}
                        >
                          {running?.id === proxy.id && running.kind === 'quality' ? (
                            <Spinner className='mr-1' />
                          ) : null}
                          {t('Quality')}
                        </Button>
```

（Delete 按钮保持原 `action` 不变。）

5. 组件 JSX 末尾（`QuickAddDialog` 之后）挂载报告弹窗：

```tsx
        <QualityReportDialog
          proxy={reportProxy}
          open={reportProxy !== null}
          onOpenChange={(open) => {
            if (!open) {
              setReportProxy(null)
            }
          }}
        />
```

6. en.json / zh.json 追加键（查重后）：`Proxy test failed`（en=`Proxy test failed`，zh=`代理测试失败`）、`Quality check failed`（en=`Quality check failed`，zh=`质量检测失败`）。

- [ ] **Step 4: 运行测试确认通过**

Run（在 `web/` 目录）: `bunx vitest run src/features/proxies`
Expected: PASS（组件 3 个 + 页面 3 个 + 原有 drawer 3 个）。

- [ ] **Step 5: typecheck + lint**

Run（在 `web/` 目录）: `bun run typecheck`
Run: `bunx oxlint -c .oxlintrc.json src/features/proxies/index.tsx src/features/proxies/components/quality-report-dialog.tsx src/features/proxies/__tests__/proxies-page.test.tsx src/features/proxies/components/__tests__/quality-report-dialog.test.tsx`
Expected: 全部通过。

- [ ] **Step 6: 提交**

```bash
git add web/src/features/proxies/index.tsx web/src/features/proxies/__tests__/proxies-page.test.tsx web/src/i18n/locales/en.json web/src/i18n/locales/zh.json
git commit -m "feat(web): add loading state, error toasts and quality report dialog to proxies page"
```

---

### Task 4: i18n 其余语言翻译（zh-TW / fr / ru / ja / vi）

**Files:**
- Modify: `web/src/i18n/locales/zh-TW.json`、`fr.json`、`ru.json`、`ja.json`、`vi.json`

**Interfaces:**
- Consumes: Task 2/3 新增的英文键。

- [ ] **Step 1: 加载 i18n-translate 技能并按其中流程执行**

Run（在 `web/` 目录）: `bun run i18n:sync`（如有该脚本，先同步/校验键）
新增键清单（en 源）：
`Quality Report`、`Score`、`Grade`、`Exit IP`、`Country`、`Latency`、`Checked at`、`Target`、`Status`、`Message`、`Base connectivity`、`Pass`、`Warn`、`Challenge`、`Fail`、`No quality results`、`Proxy test failed`、`Quality check failed`

- [ ] **Step 2: 为 5 个语言文件补译**

zh-TW：`品質檢測報告`、`分數`、`等級`、`出口 IP`、`國家`、`延遲`、`檢測時間`、`目標`、`狀態`、`訊息`、`基礎連通性`、`通過`、`警告`、`挑戰`、`失敗`、`暫無品質結果`、`代理測試失敗`、`品質檢測失敗`

fr / ru / ja / vi 按 i18n-translate 技能流程翻译（品牌名 OpenAI/Anthropic/Gemini/Grok 与 `HTTP` 不翻译）。

- [ ] **Step 3: 校验并提交**

Run（在 `web/` 目录）: `bun run i18n:sync`（或技能指定的校验命令），确认无缺失键。
Run: `bunx vitest run src/features/proxies`
Expected: PASS。

```bash
git add web/src/i18n/locales/zh-TW.json web/src/i18n/locales/fr.json web/src/i18n/locales/ru.json web/src/i18n/locales/ja.json web/src/i18n/locales/vi.json
git commit -m "i18n(web): translate proxy quality report keys for remaining locales"
```

---

### 收尾验证（非提交任务）

- [ ] 后端：`go test ./controller/ -run "TestProxy" -count=1` 全绿
- [ ] 前端：`bunx vitest run src/features/proxies` 全绿
- [ ] 前端：`bun run typecheck` 通过
- [ ] 前端生产构建：`bun run build` 成功（产物 `web/dist/`，部署提示用户）