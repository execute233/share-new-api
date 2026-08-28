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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

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

const disguiseSettings = JSON.stringify({
  disguise_enabled: true,
  fingerprint_mode: 'full',
  fingerprint_seed: 'a3f5c0d0-0000-4000-8000-000000000001',
  codex_client_version: '',
  enforce_identity: true,
})

function installApiFixtures(options?: { channel?: Record<string, unknown> }) {
  apiClient.get = async (url) => {
    if (/^\/api\/channel\/\d+$/.test(url) && options?.channel) {
      return { data: { success: true, data: options.channel } }
    }
    return { data: { success: true, data: [] } }
  }
}

function renderDisguiseDrawer(channel: Record<string, unknown>) {
  installApiFixtures({ channel })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <ChannelsProvider>
          <ChannelMutateDrawer
            open
            onOpenChange={() => undefined}
            currentRow={{ id: channel.id } as never}
          />
        </ChannelsProvider>
      </QueryClientProvider>
    </I18nextProvider>
  )
}

function disguiseChannel(id: number, settings: string): Record<string, unknown> {
  return {
    id,
    type: 62,
    name: 'disguise',
    key: '{"type":"sub2api","api_key":"sk-1"}',
    models: '',
    group: 'default',
    status: 1,
    priority: 0,
    weight: 0,
    auto_ban: 1,
    created_time: 0,
    test_time: 0,
    response_time: 0,
    used_quota: 0,
    balance_updated_time: 0,
    other: '',
    other_info: '',
    remark: '',
    max_input_tokens: 0,
    channel_info: { is_multi_key: false, multi_key_size: 0 },
    settings,
  }
}

describe('Codex disguise channel settings form', () => {
  test('shows all five disguise fields for a codex disguise channel', async () => {
    renderDisguiseDrawer(disguiseChannel(1, disguiseSettings))

    expect(await screen.findByText('Disguise Enabled')).toBeInTheDocument()
    expect(screen.getByText('Fingerprint Mode')).toBeInTheDocument()
    expect(screen.getByText('Fingerprint Seed')).toBeInTheDocument()
    expect(screen.getByText('Codex Client Version')).toBeInTheDocument()
    expect(screen.getByText('Enforce Identity')).toBeInTheDocument()
  })

  test('prefills fingerprint seed from persisted settings', async () => {
    renderDisguiseDrawer(disguiseChannel(1, disguiseSettings))

    const seedInput = await screen.findByDisplayValue(
      'a3f5c0d0-0000-4000-8000-000000000001'
    )
    expect(seedInput).toBeInTheDocument()
  })

  test('does not show disguise fields for an OpenAI channel', async () => {
    const openaiChannel = {
      ...disguiseChannel(2, '{}'),
      type: 1,
      name: 'openai',
    }
    renderDisguiseDrawer(openaiChannel)

    const user = userEvent.setup()
    await screen.findByRole('textbox', { name: 'Name *' })
    await user.click(
      screen.getByRole('button', {
        name: /Advanced Settings Request overrides/,
      })
    )

    expect(screen.queryByText('Disguise Enabled')).not.toBeInTheDocument()
    expect(screen.queryByText('Fingerprint Mode')).not.toBeInTheDocument()
  })
})