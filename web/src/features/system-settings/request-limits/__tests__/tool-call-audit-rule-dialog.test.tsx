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
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import {
  RULE_DIALOG_FORM_ID,
  ToolCallAuditRuleDialog,
  type ToolCallAuditRuleDraft,
} from '../tool-call-audit-rule-dialog'

const existingRule: ToolCallAuditRuleDraft = {
  id: 'rule-one',
  name: 'Rule one',
  enabled: true,
  severity: 'high',
  category: 'credential_access',
  tool_names: ['shell'],
  argument_paths: ['cmd'],
  match_type: 'contains',
  patterns: ['secret'],
}

function renderDialog(
  overrides: Partial<{
    open: boolean
    onOpenChange: (open: boolean) => void
    editData: ToolCallAuditRuleDraft | null
    onSave: (rule: ToolCallAuditRuleDraft) => void
  }> = {}
) {
  const onSave = vi.fn()
  const onOpenChange = vi.fn()
  render(
    <ToolCallAuditRuleDialog
      open
      onOpenChange={onOpenChange}
      onSave={onSave}
      {...overrides}
    />
  )
  return { onSave, onOpenChange }
}

function submitDialogForm() {
  const form = document.querySelector(`form#${RULE_DIALOG_FORM_ID}`)
  if (!form) throw new Error('Expected rule dialog form')
  fireEvent.submit(form)
}

function getTagInputFor(labelText: string) {
  const label = screen.getByText(labelText)
  const formItem = label.closest('[data-slot="form-item"]')
  if (!formItem) throw new Error(`Expected form item for ${labelText}`)
  return within(formItem as HTMLElement).getByRole('textbox')
}

describe('tool call audit rule dialog', () => {
  test('saves a new rule with a generated id', async () => {
    const user = userEvent.setup()
    const { onSave, onOpenChange } = renderDialog()

    await user.type(screen.getByLabelText('Rule name'), 'Block curl')
    await user.type(screen.getByPlaceholderText('Add pattern'), 'curl{Enter}')
    submitDialogForm()

    await waitFor(() => expect(onSave).toHaveBeenCalledOnce())
    const saved = onSave.mock.calls[0][0] as ToolCallAuditRuleDraft
    expect(saved.id).toMatch(/^custom-rule-/)
    expect(saved.name).toBe('Block curl')
    expect(saved.enabled).toBe(true)
    expect(saved.severity).toBe('medium')
    expect(saved.match_type).toBe('contains')
    expect(saved.patterns).toEqual(['curl'])
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  test('pre-fills an existing rule and keeps the id read-only', async () => {
    const user = userEvent.setup()
    const { onSave } = renderDialog({ editData: existingRule })

    const idInput = screen.getByLabelText('Rule ID')
    expect(idInput).toHaveValue('rule-one')
    expect(idInput).toBeDisabled()
    expect(screen.getByLabelText('Rule name')).toHaveValue('Rule one')
    expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument()

    await user.type(getTagInputFor('Patterns'), 'id_rsa{Enter}')
    submitDialogForm()

    await waitFor(() => expect(onSave).toHaveBeenCalledOnce())
    const saved = onSave.mock.calls[0][0] as ToolCallAuditRuleDraft
    expect(saved.id).toBe('rule-one')
    expect(saved.patterns).toEqual(['secret', 'id_rsa'])
  })

  test('rejects a rule without patterns', async () => {
    renderDialog()

    submitDialogForm()

    expect(
      await screen.findByText('At least one pattern is required')
    ).toBeVisible()
  })

  test('rejects an invalid argument path', async () => {
    const user = userEvent.setup()
    renderDialog()

    await user.type(getTagInputFor('Argument paths'), 'cmd/name{Enter}')
    await user.type(getTagInputFor('Patterns'), 'x{Enter}')
    submitDialogForm()

    expect(await screen.findByText('Invalid argument path')).toBeVisible()
  })

  test('rejects an uncompilable regex when match type is regex', async () => {
    const user = userEvent.setup()
    renderDialog()

    await user.type(getTagInputFor('Patterns'), '(abc{Enter}')
    await user.click(screen.getByRole('combobox', { name: 'Match type' }))
    await user.click(await screen.findByRole('option', { name: 'regex' }))
    submitDialogForm()

    expect(await screen.findByText('Invalid regex pattern')).toBeVisible()
  })
})