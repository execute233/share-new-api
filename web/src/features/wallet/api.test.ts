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
import { afterEach, describe, expect, test } from 'vitest'

const { api } = await import('@/lib/api')

const { getUserLogs } = await import('./api')

type GetMock = (url: string) => Promise<{ data: unknown }>
const apiClient = api as unknown as { get: GetMock }
const originalGet = apiClient.get

function installGetMock(): string[] {
  const requestedUrls: string[] = []
  apiClient.get = async (url) => {
    requestedUrls.push(url)
    return {
      data: {
        success: true,
        message: '',
        data: { items: [], total: 0 },
      },
    }
  }
  return requestedUrls
}

afterEach(() => {
  apiClient.get = originalGet
})

describe('getUserLogs', () => {
  test('requests the user log endpoint with page, page_size and type params', async () => {
    const requestedUrls = installGetMock()

    await getUserLogs(1, 10, 1)

    expect(requestedUrls).toEqual([
      '/api/log/self?p=1&page_size=10&type=1',
    ])
  })

  test('omits the type param when no log type is given', async () => {
    const requestedUrls = installGetMock()

    await getUserLogs(2, 20)

    expect(requestedUrls).toEqual(['/api/log/self?p=2&page_size=20'])
  })
})
