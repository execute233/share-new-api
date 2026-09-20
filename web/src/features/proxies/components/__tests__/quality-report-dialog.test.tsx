/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
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
    {
      target: 'base_connectivity',
      status: 'pass',
      http_status: 200,
      latency_ms: 123,
      message: '代理出口连通正常',
    },
    {
      target: 'openai',
      url: 'https://api.openai.com/v1/models',
      status: 'pass',
      http_status: 401,
      latency_ms: 300,
      message: '目标可达',
    },
    {
      target: 'anthropic',
      url: 'https://api.anthropic.com/v1/messages',
      status: 'challenge',
      http_status: 403,
      latency_ms: 500,
      message: '目标返回 Cloudflare 挑战',
      cf_ray: 'abc123',
    },
    {
      target: 'gemini',
      url: 'https://generativelanguage.googleapis.com/$discovery/rest?version=v1beta',
      status: 'fail',
      latency_ms: 1000,
      message: '探测请求失败: context deadline exceeded',
    },
    {
      target: 'grok',
      url: 'https://api.x.ai/v1/models',
      status: 'warn',
      http_status: 429,
      latency_ms: 800,
      message: '目标被限流',
    },
  ],
}

describe('QualityReportDialog', () => {
  test('renders summary and per-target rows with brand name, url and status', () => {
    render(
      <QualityReportDialog
        proxy={baseProxy}
        open
        onOpenChange={() => undefined}
      />
    )

    expect(screen.getByText('Quality Report')).toBeInTheDocument()
    expect(screen.getByText('70')).toBeInTheDocument()
    expect(screen.getByText(/Grade: B/)).toBeInTheDocument()
    expect(
      screen.getByText('通过 1 项，告警 1 项，失败 2 项，挑战 1 项')
    ).toBeInTheDocument()
    expect(screen.getByText(/203\.0\.113\.8/)).toBeInTheDocument()
    expect(screen.getByText(/Testland/)).toBeInTheDocument()
    expect(screen.getByText(/Latency: 123ms/)).toBeInTheDocument()

    expect(screen.getByText('Base connectivity')).toBeInTheDocument()
    expect(screen.getByText('OpenAI')).toBeInTheDocument()
    expect(
      screen.getByText('https://api.openai.com/v1/models')
    ).toBeInTheDocument()
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
    expect(
      screen.getByText('探测请求失败: context deadline exceeded')
    ).toBeInTheDocument()
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
      <QualityReportDialog
        proxy={emptyProxy}
        open
        onOpenChange={() => undefined}
      />
    )

    expect(screen.getByText('No quality results')).toBeInTheDocument()
    expect(screen.getByText(/Exit IP: -/)).toBeInTheDocument()
    expect(screen.getByText(/Country: -/)).toBeInTheDocument()
    expect(screen.getByText(/Latency: -/)).toBeInTheDocument()
    expect(screen.getByText(/Checked at: -/)).toBeInTheDocument()
  })

  test('renders nothing when proxy is null', () => {
    render(
      <QualityReportDialog proxy={null} open onOpenChange={() => undefined} />
    )
    expect(screen.queryByText('Quality Report')).not.toBeInTheDocument()
  })
})
