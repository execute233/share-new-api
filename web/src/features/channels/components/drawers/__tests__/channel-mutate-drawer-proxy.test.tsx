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
import {
  createRouter,
  createRootRoute,
  createMemoryHistory,
  RouterContextProvider,
} from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

vi.mock('@/lib/lobe-icon', () => ({
  getLobeIcon: () => null,
}))

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { ChannelsProvider } = await import('../../channels-provider')
const { ChannelMutateDrawer } = await import('../channel-mutate-drawer')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = { get: ApiMethod }
const apiClient = api as unknown as MockableApi

const originalAuth = useAuthStore.getState().auth
beforeEach(() => {
  useAuthStore.setState({
    auth: {
      ...originalAuth,
      user: { id: 1, username: 'root', role: ROLE.SUPER_ADMIN },
    },
  })
})
afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.setState({ auth: originalAuth })
})

const proxyUs = {
  id: 1,
  name: 'proxy-us',
  protocol: 'http',
  host: 'us.example.com',
  port: 8080,
  status: 'active',
  credential_configured: false,
  credential_decrypt_failed: false,
  bound_channel_count: 0,
}

const proxyEu = {
  id: 2,
  name: 'proxy-eu',
  protocol: 'http',
  host: 'eu.example.com',
  port: 8080,
  status: 'active',
  credential_configured: false,
  credential_decrypt_failed: false,
  bound_channel_count: 0,
}

function installApiFixtures(options?: { channel?: Record<string, unknown> }) {
  vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
    if (url === '/api/proxy/all') {
      return { data: { success: true, data: [proxyUs, proxyEu] } }
    }
    if (/^\/api\/channel\/\d+$/.test(url) && options?.channel) {
      return { data: { success: true, data: options.channel } }
    }
    return { data: { success: true, data: [] } }
  })
}

function renderDrawer(options?: { channel?: Record<string, unknown> }) {
  installApiFixtures(options)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const router = createRouter({
    routeTree: createRootRoute(),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <RouterContextProvider router={router}>
          <ChannelsProvider>
            <ChannelMutateDrawer
              open
              onOpenChange={() => undefined}
              currentRow={
                options?.channel ? ({ id: options.channel.id } as never) : null
              }
            />
          </ChannelsProvider>
        </RouterContextProvider>
      </QueryClientProvider>
    </I18nextProvider>
  )
}

async function openProxySelect() {
  const user = userEvent.setup()
  await user.click(await screen.findByRole('option', { name: /^OpenAI / }))
  await screen.findByRole('textbox', { name: 'Name *' })
  await user.click(
    screen.getByRole('tab', {
      name: /Other Settings/,
    })
  )
  const proxySelect = await screen.findByRole('combobox', { name: 'Proxy' })
  await user.click(proxySelect)
}

describe('Channel mutate drawer proxy selection', () => {
  test('lists active proxies from the proxy pool in the Proxy dropdown', async () => {
    renderDrawer()
    await openProxySelect()

    expect(await screen.findByRole('option', { name: 'proxy-us' })).toBeTruthy()
    expect(await screen.findByRole('option', { name: 'proxy-eu' })).toBeTruthy()
    expect(
      screen.getByRole('option', { name: 'Direct (No Proxy)' })
    ).toBeTruthy()
  })

  test('selecting a proxy updates the combobox value', async () => {
    renderDrawer()
    await openProxySelect()
    const user = userEvent.setup()

    await user.click(await screen.findByRole('option', { name: 'proxy-us' }))

    expect(screen.getByRole('combobox', { name: 'Proxy' })).toHaveTextContent(
      'proxy-us'
    )
  })

  test('keeps showing the currently bound proxy when it is no longer in the active pool', async () => {
    renderDrawer({
      channel: {
        id: 5,
        type: 1,
        key: 'sk-test',
        status: 1,
        name: 'ch1',
        created_time: 0,
        test_time: 0,
        response_time: 0,
        balance_updated_time: 0,
        proxy_id: 7,
        channel_info: {
          is_multi_key: false,
          multi_key_size: 0,
          multi_key_polling_index: 0,
          multi_key_mode: 'random',
        },
      },
    })
    const user = userEvent.setup()
    await screen.findByRole('textbox', { name: 'Name *' })
    await user.click(
      screen.getByRole('tab', {
        name: /Other Settings/,
      })
    )

    expect(
      await screen.findByRole('combobox', { name: 'Proxy' })
    ).toHaveTextContent('#7')
  })
})
