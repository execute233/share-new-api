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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'

import * as systemSettingsApi from '../../api'
import { ToolCallAuditSection } from '../tool-call-audit-section'

const config = JSON.stringify({
  version: 1,
  mode: 'disabled',
  channel_ids: [],
  rules: [
    {
      id: 'rule-one',
      name: 'Rule one',
      enabled: true,
      severity: 'high',
      category: 'credential_access',
      tool_names: [],
      argument_paths: [],
      match_type: 'contains',
      patterns: ['secret'],
    },
  ],
  max_argument_bytes: 65536,
  max_total_argument_bytes: 262144,
  max_json_depth: 32,
  log_retention_days: 7,
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('tool call audit rule editor', () => {
  test('allows an administrator to disable and delete a rule visually', () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <ToolCallAuditSection defaultValues={config} />
      </QueryClientProvider>
    )

    const enabledSwitch = screen.getByRole('switch', { name: 'Enabled' })
    expect(enabledSwitch).toBeChecked()
    fireEvent.click(enabledSwitch)
    expect(enabledSwitch).not.toBeChecked()

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(screen.queryByText('Rule one')).not.toBeInTheDocument()

    queryClient.clear()
  })

  test.each(['abc', '1,abc,2', '1,0', '1,2.5'])(
    'shows a validation error for invalid channel ID input %s',
    async (channelIds) => {
      const queryClient = new QueryClient({
        defaultOptions: { mutations: { retry: false } },
      })
      const user = userEvent.setup()
      const { container } = render(
        <QueryClientProvider client={queryClient}>
          <ToolCallAuditSection defaultValues={config} />
        </QueryClientProvider>
      )

      const channelInput = screen.getByRole('textbox', {
        name: 'Channel IDs',
      })
      await user.type(channelInput, channelIds)
      const settingsForm = container.querySelector('form')
      if (!settingsForm) throw new Error('Expected tool call audit form')
      fireEvent.submit(settingsForm)

      expect(
        await screen.findByText(
          'Channel IDs must be comma-separated positive integers'
        )
      ).toBeVisible()
      expect(channelInput).toHaveAttribute('aria-invalid', 'true')

      queryClient.clear()
    }
  )

  test('restores the current default rules from the backend', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    const user = userEvent.setup()
    const getDefaults = vi
      .spyOn(systemSettingsApi, 'getToolCallAuditDefaults')
      .mockResolvedValue({
        success: true,
        message: '',
        data: {
          rules: [
            {
              id: 'server-default',
              name: 'Server default rule',
              enabled: true,
              severity: 'critical',
              category: 'credential_access',
              tool_names: [],
              argument_paths: [],
              match_type: 'contains',
              patterns: ['git-credentials'],
            },
          ],
        },
      })

    render(
      <QueryClientProvider client={queryClient}>
        <ToolCallAuditSection defaultValues={config} />
      </QueryClientProvider>
    )

    expect(getDefaults).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Restore defaults' }))

    await waitFor(() => expect(getDefaults).toHaveBeenCalledOnce())
    expect(await screen.findByText('Server default rule')).toBeVisible()
    expect(screen.queryByText('Rule one')).not.toBeInTheDocument()

    queryClient.clear()
  })

  test('keeps the draft rules when loading defaults fails', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    const user = userEvent.setup()
    const getDefaults = vi
      .spyOn(systemSettingsApi, 'getToolCallAuditDefaults')
      .mockRejectedValue(new Error('defaults unavailable'))

    render(
      <QueryClientProvider client={queryClient}>
        <ToolCallAuditSection defaultValues={config} />
      </QueryClientProvider>
    )

    const restoreButton = screen.getByRole('button', {
      name: 'Restore defaults',
    })
    await user.click(restoreButton)

    await waitFor(() => expect(getDefaults).toHaveBeenCalledOnce())
    await waitFor(() => expect(restoreButton).toBeEnabled())
    await waitFor(() => expect(screen.getByText('Rule one')).toBeVisible())

    queryClient.clear()
  })

  test('adds a rule through the rule dialog', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    const user = userEvent.setup()

    render(
      <QueryClientProvider client={queryClient}>
        <ToolCallAuditSection defaultValues={config} />
      </QueryClientProvider>
    )

    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.type(screen.getByLabelText('Rule name'), 'Block curl')
    await user.type(screen.getByPlaceholderText('Add pattern'), 'curl{Enter}')
    const dialogForm = document.querySelector('form#tool-call-audit-rule-form')
    if (!dialogForm) throw new Error('Expected rule dialog form')
    fireEvent.submit(dialogForm)

    expect(await screen.findByText('Block curl')).toBeVisible()
    await waitFor(() =>
      expect(document.querySelector('form#tool-call-audit-rule-form')).toBeNull()
    )

    queryClient.clear()
  })

  test('edits a rule through the rule dialog', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    const user = userEvent.setup()

    render(
      <QueryClientProvider client={queryClient}>
        <ToolCallAuditSection defaultValues={config} />
      </QueryClientProvider>
    )

    await user.click(screen.getByRole('button', { name: 'Edit' }))
    const nameInput = screen.getByLabelText('Rule name')
    expect(nameInput).toHaveValue('Rule one')
    await user.clear(nameInput)
    await user.type(nameInput, 'Rule one renamed')
    const dialogForm = document.querySelector('form#tool-call-audit-rule-form')
    if (!dialogForm) throw new Error('Expected rule dialog form')
    fireEvent.submit(dialogForm)

    expect(await screen.findByText('Rule one renamed')).toBeVisible()
    expect(screen.queryByText('Rule one')).not.toBeInTheDocument()

    queryClient.clear()
  })

  test('blocks switching to visual mode when the rules JSON is invalid', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    const user = userEvent.setup()

    render(
      <QueryClientProvider client={queryClient}>
        <ToolCallAuditSection defaultValues={config} />
      </QueryClientProvider>
    )

    await user.click(screen.getByRole('button', { name: 'Switch to JSON' }))
    const jsonEditor = screen.getByRole('textbox', {
      name: 'Tool audit rules JSON',
    })
    fireEvent.input(jsonEditor, { target: { value: '{invalid json' } })
    await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))

    expect(
      screen.getByRole('textbox', { name: 'Tool audit rules JSON' })
    ).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Add rule' })
    ).not.toBeInTheDocument()

    queryClient.clear()
  })
})
