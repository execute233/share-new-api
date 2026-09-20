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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ProxyMutateDrawer } = await import('../proxy-mutate-drawer')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const FIELD_LABELS = [
  'Name',
  'Protocol',
  'Host',
  'Port',
  'Username',
  'Password',
  'Status',
] as const

function findLabel(text: string): HTMLLabelElement {
  const label = screen.getByText(text).closest('label')
  if (!label) {
    throw new Error(`Expected label "${text}"`)
  }
  return label
}

function renderDrawer(): void {
  render(
    <I18nextProvider i18n={i18n}>
      <ProxyMutateDrawer
        proxy={null}
        open
        onOpenChange={() => undefined}
        onSaved={() => undefined}
      />
    </I18nextProvider>
  )
}

describe('Proxy mutate drawer field labels', () => {
  test('every field label owns its control and keeps the control outside the label element, so label text is never squeezed onto one column', async () => {
    renderDrawer()
    await waitFor(() => {
      expect(findLabel(FIELD_LABELS[0])).toBeTruthy()
    })

    for (const labelText of FIELD_LABELS) {
      const label = findLabel(labelText)
      const control = label.control
      expect(
        control,
        `Expected a control associated with "${labelText}"`
      ).not.toBeNull()
      expect(
        label.querySelector('input, textarea, button[role="combobox"]'),
        `"${labelText}" control must not be nested inside the label element`
      ).toBeNull()
    }
  })
})

describe('Proxy mutate drawer shadowsocks', () => {
  test('lists ss protocol option', async () => {
    renderDrawer()
    await waitFor(() => {
      expect(findLabel('Protocol')).toBeTruthy()
    })
    const user = userEvent.setup()
    await user.click(screen.getByRole('combobox', { name: 'Protocol' }))
    expect(await screen.findByRole('option', { name: 'ss' })).toBeTruthy()
  })

  test('switches username field to encryption method with method placeholder when ss is selected', async () => {
    renderDrawer()
    await waitFor(() => {
      expect(findLabel('Protocol')).toBeTruthy()
    })
    const user = userEvent.setup()
    await user.click(screen.getByRole('combobox', { name: 'Protocol' }))
    await user.click(await screen.findByRole('option', { name: 'ss' }))

    expect(findLabel('Encryption method')).toBeTruthy()
    expect(screen.getByPlaceholderText('chacha20-ietf-poly1305')).toBeTruthy()
  })
})
