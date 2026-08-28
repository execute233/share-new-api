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
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { createProxy, updateProxy } = await import('../../api')
const { ProxyMutateDrawer } = await import('../proxy-mutate-drawer')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

vi.mock('../../api', () => ({
  createProxy: vi.fn(),
  updateProxy: vi.fn(),
}))

const mockCreateProxy = vi.mocked(createProxy)
const mockUpdateProxy = vi.mocked(updateProxy)

function renderDrawer(options?: { proxy?: unknown }) {
  render(
    <I18nextProvider i18n={i18n}>
      <ProxyMutateDrawer
        proxy={(options?.proxy as never) ?? null}
        open
        onOpenChange={() => undefined}
        onSaved={() => undefined}
      />
    </I18nextProvider>
  )
}

async function fillAndSave() {
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('Name'), 'proxy-us')
  await user.type(screen.getByLabelText('Host'), 'us.example.com')
  const port = screen.getByLabelText('Port')
  await user.clear(port)
  await user.type(port, '8080')
  await user.click(screen.getByRole('button', { name: 'Save' }))
}

describe('Proxy mutate drawer validation', () => {
  test('rejects a port outside the valid range and does not call the API', async () => {
    renderDrawer()
    const user = userEvent.setup()
    await user.type(screen.getByLabelText('Name'), 'proxy-us')
    await user.type(screen.getByLabelText('Host'), 'us.example.com')
    const port = screen.getByLabelText('Port')
    await user.clear(port)
    await user.type(port, '70000')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      await screen.findByText('Port must be between 1 and 65535')
    ).toBeTruthy()
    expect(mockCreateProxy).not.toHaveBeenCalled()
  })

  test('requires a name before saving', async () => {
    renderDrawer()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('Name is required')).toBeTruthy()
    expect(mockCreateProxy).not.toHaveBeenCalled()
  })

  test('submits valid values to the API', async () => {
    mockCreateProxy.mockResolvedValue({ success: true })
    renderDrawer()
    await fillAndSave()

    await vi.waitFor(() => {
      expect(mockCreateProxy).toHaveBeenCalledTimes(1)
    })
    expect(mockCreateProxy.mock.calls[0][0]).toMatchObject({
      name: 'proxy-us',
      host: 'us.example.com',
      port: 8080,
      protocol: 'http',
      status: 'active',
    })
  })

  test('submits edited values through updateProxy', async () => {
    mockUpdateProxy.mockResolvedValue({ success: true })
    renderDrawer({
      proxy: {
        id: 3,
        name: 'old-name',
        protocol: 'http',
        host: 'old.example.com',
        port: 8080,
        status: 'active',
        credential_configured: false,
        credential_decrypt_failed: false,
        bound_channel_count: 0,
      },
    })
    const user = userEvent.setup()
    const name = screen.getByLabelText('Name')
    await user.clear(name)
    await user.type(name, 'new-name')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await vi.waitFor(() => {
      expect(mockUpdateProxy).toHaveBeenCalledTimes(1)
    })
    expect(mockUpdateProxy.mock.calls[0][0]).toBe(3)
    expect(mockUpdateProxy.mock.calls[0][1]).toMatchObject({ name: 'new-name' })
  })
})