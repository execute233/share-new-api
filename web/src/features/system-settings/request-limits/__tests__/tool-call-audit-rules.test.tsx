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
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

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
})
