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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, render } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import * as opsApi from '../api'
import { OpsDetails } from '../details'

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

it('stops polling request details when monitoring is paused', async () => {
  vi.useFakeTimers()
  const getEvents = vi.spyOn(opsApi, 'getOpsEvents').mockResolvedValue({
    items: [],
    has_more: false,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const filters = { channel_id: '', model: '', group: '' }
  const range = { start: 1_790_121_600, end: 1_790_121_660 }
  const view = (paused: boolean) => (
    <QueryClientProvider client={client}>
      <OpsDetails filters={filters} range={range} paused={paused} />
    </QueryClientProvider>
  )

  const { rerender } = render(view(false))
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0)
  })
  expect(getEvents).toHaveBeenCalledTimes(1)

  await act(async () => {
    await vi.advanceTimersByTimeAsync(10_000)
  })
  expect(getEvents).toHaveBeenCalledTimes(2)

  rerender(view(true))
  await act(async () => {
    await vi.advanceTimersByTimeAsync(20_000)
  })
  expect(getEvents).toHaveBeenCalledTimes(2)
})
