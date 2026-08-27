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

For commercial licensing, please contact support@quantumnous.com.
*/
import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

const { api } = await import('@/lib/api')
const { Wallet } = await import('../index')

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  post: ApiMethod
}

const apiClient = api as unknown as MockableApi

function installApiFixtures(): { selfCalls: () => number } {
  let selfCalls = 0
  apiClient.get = async (url) => {
    if (url === '/api/user/self') {
      selfCalls += 1
      return {
        data: {
          success: true,
          data: {
            id: 1,
            username: 'test-user',
            quota: 1000,
            used_quota: 200,
            request_count: 5,
            aff_quota: 0,
            aff_history_quota: 0,
            aff_count: 0,
            group: 'default',
          },
        },
      }
    }
    if (url === '/api/subscription/plans') {
      return { data: { success: true, data: [] } }
    }
    if (url === '/api/subscription/self') {
      return {
        data: {
          success: true,
          data: {
            billing_preference: 'subscription_first',
            subscriptions: [],
            all_subscriptions: [],
          },
        },
      }
    }
    if (url === '/api/user/aff') {
      return { data: { success: true, data: 'https://example.com/r/abc' } }
    }
    if (url.startsWith('/api/log/self?')) {
      return { data: { success: true, data: { items: [], total: 0 } } }
    }
    throw new Error(`Unexpected GET ${url}`)
  }
  apiClient.post = async (url) => {
    throw new Error(`Unexpected POST ${url}`)
  }
  return { selfCalls: () => selfCalls }
}

describe('Wallet page', () => {
  test('loads the current user and renders balance stats on mount', async () => {
    const { selfCalls } = installApiFixtures()

    render(<Wallet />)

    expect(await screen.findByText('Current Balance')).toBeInTheDocument()
    expect(screen.getByText('Total Usage')).toBeInTheDocument()
    expect(screen.getByText('API Requests')).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
    expect(selfCalls()).toBeGreaterThanOrEqual(1)
  })
})
