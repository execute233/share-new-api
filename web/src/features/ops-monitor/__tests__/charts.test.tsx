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
import { expect, it, vi } from 'vitest'

import type { Snapshot } from '../api'
import { OpsTrend } from '../charts'

vi.mock('recharts', async () => {
  const React = await import('react')
  const passthrough = ({ children }: { children?: React.ReactNode }) => children
  return {
    ResponsiveContainer: passthrough,
    LineChart: passthrough,
    CartesianGrid: () => null,
    XAxis: () => null,
    YAxis: () => null,
    Line: () => null,
    Legend: () => null,
    Tooltip: ({
      content,
    }: {
      content: React.ReactElement<Record<string, unknown>>
    }) =>
      React.cloneElement(content, {
        active: true,
        payload: [
          {
            dataKey: 'qps',
            name: 'qps',
            value: 1,
            payload: { time: 1_790_121_600_000 },
          },
        ],
      }),
  }
})

it('shows an active trend tooltip without treating the series name as a date', () => {
  const snapshot = {
    start: 1_790_121_600,
    end: 1_790_121_660,
    step: 60,
    started_at: 1_790_121_600,
    generated_at: 1_790_121_660,
    trend: [
      {
        bucket: 1_790_121_600,
        requests: 60,
        tokens: 120,
        unknown_tokens: 0,
        retries: 0,
      },
    ],
  } as Snapshot

  render(<OpsTrend snapshot={snapshot} />)

  expect(screen.getByText('QPS')).toBeInTheDocument()
})
