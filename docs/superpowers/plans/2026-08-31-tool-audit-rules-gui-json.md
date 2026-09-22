# Tool Audit Rules GUI/JSON Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the "Rules" block of the tool call audit settings section so it toggles between a GUI mode (list + per-rule centered Dialog editor) and a JSON mode (code editor), blocking JSON→GUI switching on invalid JSON.

**Architecture:** A new `ToolCallAuditRuleDialog` component (pattern: `rate-limit-dialog.tsx` — centered `Dialog` + react-hook-form + zod, fully aligned with backend `ValidateToolCallAuditSettings`) edits a single rule. `ToolCallAuditSection` gains an `editMode: 'visual' | 'json'` state; both modes read/write the same existing `rules` form field via the existing `setRules`/`parseRules` helpers, so mode switches stay consistent. JSON→GUI switching validates the JSON first and refuses with a toast on failure.

**Tech Stack:** React 19, TypeScript, react-hook-form + zod, Base UI (Dialog, Select, Switch), `TagInput` (`@/components/tag-input.tsx`), `JsonCodeEditor` (`@/components/json-code-editor.tsx`), Vitest + React Testing Library, Bun.

## Global Constraints

- Every new/changed file keeps the AGPL license header (copy the header from any existing file in `web/src/features/system-settings/`). `bun run format` / `bun run copyright:check` enforce this.
- All user-facing strings go through `t()` with English source strings as i18n keys (en.json is the source of truth). Do NOT add inline English literals outside `t()`.
- Tests must live in `web/src/features/system-settings/request-limits/__tests__/` and use `@testing-library/react` + `vitest` (existing style: `userEvent` for interaction, `fireEvent.submit(form)` for form submission).
- Run `bun run typecheck`, `bun run lint`, and the affected test files before declaring any task complete.
- Do NOT commit the design/plan documents under `docs/superpowers/` (user instruction). Code commits are fine.
- Do not modify the backend (`setting/tool_call_audit.go`) or the top GUI fields (mode/channel IDs/limits) of the section.

---

### Task 1: ToolCallAuditRuleDialog component

**Files:**
- Create: `web/src/features/system-settings/request-limits/tool-call-audit-rule-dialog.tsx`
- Test: `web/src/features/system-settings/request-limits/__tests__/tool-call-audit-rule-dialog.test.tsx`

**Interfaces:**
- Produces:
  - `export type ToolCallAuditRuleDraft = { id?: string; name?: string; enabled?: boolean; severity?: string; category?: string; match_type?: string; tool_names?: string[]; argument_paths?: string[]; patterns?: string[]; [key: string]: unknown }` — the loose rule shape shared with the section (parsed from JSON).
  - `export const RULE_DIALOG_FORM_ID = 'tool-call-audit-rule-form'` — form id used by tests.
  - `export function ToolCallAuditRuleDialog(props: { open: boolean; onOpenChange: (open: boolean) => void; editData?: ToolCallAuditRuleDraft | null; onSave: (rule: ToolCallAuditRuleDraft) => void })` — renders a centered Dialog; submit button label is `t('Add')` for new rules, `t('Update')` for edits.

- [ ] **Step 1: Write the failing tests**

Create `web/src/features/system-settings/request-limits/__tests__/tool-call-audit-rule-dialog.test.tsx`:

```tsx
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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
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

describe('tool call audit rule dialog', () => {
  test('saves a new rule with a generated id', async () => {
    const user = userEvent.setup()
    const { onSave } = renderDialog()

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

    await user.type(screen.getByPlaceholderText('Add pattern'), 'id_rsa{Enter}')
    submitDialogForm()

    await waitFor(() => expect(onSave).toHaveBeenCalledOnce())
    const saved = onSave.mock.calls[0][0] as ToolCallAuditRuleDraft
    expect(saved.id).toBe('rule-one')
    expect(saved.patterns).toEqual(['secret', 'id_rsa'])
  })

  test('rejects a rule without patterns', () => {
    renderDialog()

    submitDialogForm()

    expect(screen.getByText('At least one pattern is required')).toBeVisible()
  })

  test('rejects an invalid argument path', async () => {
    const user = userEvent.setup()
    renderDialog()

    await user.type(
      screen.getByPlaceholderText('Add argument path'),
      'cmd/name{Enter}'
    )
    await user.type(screen.getByPlaceholderText('Add pattern'), 'x{Enter}')
    submitDialogForm()

    expect(screen.getByText('Invalid argument path')).toBeVisible()
  })

  test('rejects an uncompilable regex when match type is regex', async () => {
    const user = userEvent.setup()
    renderDialog()

    await user.type(screen.getByPlaceholderText('Add pattern'), '(abc{Enter}')
    await user.click(screen.getByRole('combobox', { name: 'Match type' }))
    await user.click(await screen.findByRole('option', { name: 'regex' }))
    submitDialogForm()

    expect(screen.getByText('Invalid regex pattern')).toBeVisible()
  })
})
```

Note: if `userEvent.click` on the combobox proves flaky in jsdom, fall back to `fireEvent.click` on the combobox trigger and `fireEvent.click` on the option.

- [ ] **Step 2: Run the tests to verify they fail**

Run (from `web/`):
```
bun run test --request-limits/__tests__/tool-call-audit-rule-dialog.test.tsx
```
Expected: FAIL — module `../tool-call-audit-rule-dialog` cannot be resolved.

- [ ] **Step 3: Implement the dialog**

Create `web/src/features/system-settings/request-limits/tool-call-audit-rule-dialog.tsx`:

```tsx
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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Dialog } from '@/components/dialog'
import { TagInput } from '@/components/tag-input'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

export type ToolCallAuditRuleDraft = {
  id?: string
  name?: string
  enabled?: boolean
  severity?: string
  category?: string
  match_type?: string
  tool_names?: string[]
  argument_paths?: string[]
  patterns?: string[]
  [key: string]: unknown
}

const SEVERITY_OPTIONS = ['low', 'medium', 'high', 'critical'] as const
const CATEGORY_OPTIONS = [
  'credential_access',
  'sensitive_path',
  'environment_dump',
  'dangerous_shell',
  'network_exfiltration',
  'ssrf',
  'encoded_command',
  'reverse_shell',
  'secret_pattern',
] as const
const MATCH_TYPE_OPTIONS = [
  'contains',
  'exact',
  'glob',
  'regex',
  'keyword_set',
] as const

const ARGUMENT_PATH_PATTERN = /^[A-Za-z0-9_*-]+(?:\.[A-Za-z0-9_*-]+)*$/

const ruleDialogSchema = z
  .object({
    id: z
      .string()
      .min(1, 'Rule ID is required')
      .max(128, 'Rule ID is too long'),
    name: z.string().max(256, 'Rule name is too long'),
    enabled: z.boolean(),
    severity: z
      .string()
      .refine(
        (value) => (SEVERITY_OPTIONS as readonly string[]).includes(value),
        'Invalid severity'
      ),
    category: z
      .string()
      .refine(
        (value) => (CATEGORY_OPTIONS as readonly string[]).includes(value),
        'Invalid category'
      ),
    match_type: z
      .string()
      .refine(
        (value) => (MATCH_TYPE_OPTIONS as readonly string[]).includes(value),
        'Invalid match type'
      ),
    tool_names: z
      .array(z.string().max(256, 'Tool name is too long'))
      .max(128, 'Too many tool names'),
    argument_paths: z
      .array(z.string().max(512, 'Argument path is too long'))
      .max(128, 'Too many argument paths')
      .refine(
        (paths) => paths.every((path) => ARGUMENT_PATH_PATTERN.test(path)),
        'Invalid argument path'
      ),
    patterns: z
      .array(
        z
          .string()
          .min(1, 'Pattern cannot be empty')
          .max(4096, 'Pattern is too long')
      )
      .min(1, 'At least one pattern is required')
      .max(128, 'Too many patterns'),
  })
  .superRefine((values, ctx) => {
    if (values.match_type !== 'regex') return
    values.patterns.forEach((pattern, index) => {
      try {
        new RegExp(pattern)
      } catch {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['patterns', index],
          message: 'Invalid regex pattern',
        })
      }
    })
  })

type RuleDialogFormValues = z.infer<typeof ruleDialogSchema>

export const RULE_DIALOG_FORM_ID = 'tool-call-audit-rule-form'

type ToolCallAuditRuleDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  editData?: ToolCallAuditRuleDraft | null
  onSave: (rule: ToolCallAuditRuleDraft) => void
}

function newRuleId() {
  return `custom-rule-${globalThis.crypto.randomUUID()}`
}

function pickKnownOption(
  options: readonly string[],
  value: string | undefined,
  fallback: string
) {
  return value !== undefined && options.includes(value) ? value : fallback
}

export function ToolCallAuditRuleDialog({
  open,
  onOpenChange,
  editData,
  onSave,
}: ToolCallAuditRuleDialogProps) {
  const { t } = useTranslation()
  const isEditMode = !!editData

  const form = useForm<RuleDialogFormValues>({
    resolver: zodResolver(ruleDialogSchema),
    defaultValues: {
      id: '',
      name: '',
      enabled: true,
      severity: 'medium',
      category: 'secret_pattern',
      match_type: 'contains',
      tool_names: [],
      argument_paths: [],
      patterns: [],
    },
  })

  useEffect(() => {
    if (!open) return
    form.reset(
      editData
        ? {
            id: editData.id ?? '',
            name: editData.name ?? '',
            enabled: editData.enabled ?? true,
            severity: pickKnownOption(
              SEVERITY_OPTIONS,
              editData.severity,
              'medium'
            ),
            category: pickKnownOption(
              CATEGORY_OPTIONS,
              editData.category,
              'secret_pattern'
            ),
            match_type: pickKnownOption(
              MATCH_TYPE_OPTIONS,
              editData.match_type,
              'contains'
            ),
            tool_names: Array.isArray(editData.tool_names)
              ? editData.tool_names
              : [],
            argument_paths: Array.isArray(editData.argument_paths)
              ? editData.argument_paths
              : [],
            patterns: Array.isArray(editData.patterns)
              ? editData.patterns
              : [],
          }
        : {
            id: newRuleId(),
            name: '',
            enabled: true,
            severity: 'medium',
            category: 'secret_pattern',
            match_type: 'contains',
            tool_names: [],
            argument_paths: [],
            patterns: [],
          }
    )
  }, [editData, form, open])

  const handleSubmit = (values: RuleDialogFormValues) => {
    onSave(values)
    form.reset()
    onOpenChange(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={isEditMode ? t('Edit rule') : t('Add rule')}
      description={t('Configure a tool call audit rule.')}
      contentClassName='sm:max-w-[560px]'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={RULE_DIALOG_FORM_ID}>
            {isEditMode ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={RULE_DIALOG_FORM_ID}
          onSubmit={form.handleSubmit(handleSubmit)}
          className='space-y-4'
        >
          <FormField
            control={form.control}
            name='id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Rule ID')}</FormLabel>
                <FormControl>
                  <Input {...field} disabled={isEditMode} />
                </FormControl>
                <FormDescription>
                  {t('Auto-generated when adding a new rule.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Rule name')}</FormLabel>
                <FormControl>
                  <Input {...field} placeholder={t('Block SSH private key access')} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3'>
                <FormLabel>{t('Enabled')}</FormLabel>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    aria-label={t('Enabled')}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='severity'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Severity')}</FormLabel>
                  <Select
                    items={SEVERITY_OPTIONS.map((value) => ({
                      value,
                      label: value,
                    }))}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {SEVERITY_OPTIONS.map((value) => (
                          <SelectItem key={value} value={value}>
                            {value}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='category'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Category')}</FormLabel>
                  <Select
                    items={CATEGORY_OPTIONS.map((value) => ({
                      value,
                      label: value,
                    }))}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {CATEGORY_OPTIONS.map((value) => (
                          <SelectItem key={value} value={value}>
                            {value}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='match_type'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Match type')}</FormLabel>
                <Select
                  items={MATCH_TYPE_OPTIONS.map((value) => ({
                    value,
                    label: value,
                  }))}
                  value={field.value}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {MATCH_TYPE_OPTIONS.map((value) => (
                        <SelectItem key={value} value={value}>
                          {value}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='tool_names'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Tool names')}</FormLabel>
                <FormControl>
                  <TagInput
                    value={field.value}
                    onChange={field.onChange}
                    placeholder={t('Add tool name')}
                  />
                </FormControl>
                <FormDescription>
                  {t('Tool names matched against this rule. Empty matches all tools.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='argument_paths'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Argument paths')}</FormLabel>
                <FormControl>
                  <TagInput
                    value={field.value}
                    onChange={field.onChange}
                    placeholder={t('Add argument path')}
                  />
                </FormControl>
                <FormDescription>
                  {t('Argument paths to inspect within the tool arguments.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='patterns'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Patterns')}</FormLabel>
                <FormControl>
                  <TagInput
                    value={field.value}
                    onChange={field.onChange}
                    placeholder={t('Add pattern')}
                  />
                </FormControl>
                <FormDescription>
                  {t('Patterns matched against the tool arguments.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `web/`):
```
bun run test --request-limits/__tests__/tool-call-audit-rule-dialog.test.tsx
```
Expected: PASS (5 tests).

- [ ] **Step 5: Typecheck and lint the new files**

Run (from `web/`):
```
bun run typecheck
bun run lint
```
Expected: both exit 0.

- [ ] **Step 6: Commit**

```bash
git add web/src/features/system-settings/request-limits/tool-call-audit-rule-dialog.tsx web/src/features/system-settings/request-limits/__tests__/tool-call-audit-rule-dialog.test.tsx
git commit -m "feat: add GUI dialog for editing single tool call audit rules"
```

---

### Task 2: Wire mode toggle and dialog into ToolCallAuditSection

**Files:**
- Modify: `web/src/features/system-settings/request-limits/tool-call-audit-section.tsx`
- Test: `web/src/features/system-settings/request-limits/__tests__/tool-call-audit-rules.test.tsx`

**Interfaces:**
- Consumes: `ToolCallAuditRuleDialog`, `RULE_DIALOG_FORM_ID`, `ToolCallAuditRuleDraft` from `./tool-call-audit-rule-dialog` (Task 1).
- Produces: section behavior — Rules block header shows `Restore defaults`, `Add rule` (GUI mode only), and the `Switch to JSON`/`Switch to Visual` toggle; GUI mode shows the rule list with Edit buttons; JSON mode shows only the `JsonCodeEditor`; invalid JSON blocks switching back to GUI with a toast.

- [ ] **Step 1: Add failing section tests**

Append to `web/src/features/system-settings/request-limits/__tests__/tool-call-audit-rules.test.tsx` (existing tests must keep passing — GUI is the default mode):

```tsx
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
  fireEvent.change(jsonEditor, { target: { value: '{invalid json' } })
  await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))

  expect(
    screen.getByRole('textbox', { name: 'Tool audit rules JSON' })
  ).toBeVisible()
  expect(screen.queryByRole('button', { name: 'Add rule' })).not.toBeInTheDocument()

  queryClient.clear()
})
```

Note: if `getByRole('textbox', { name: 'Tool audit rules JSON' })` fails to find the Yace textarea, fall back to `document.querySelector('.json-code-editor-textarea')` and cast to `HTMLTextAreaElement` before `fireEvent.change`.

- [ ] **Step 2: Run the tests to verify they fail**

Run (from `web/`):
```
bun run test --request-limits/__tests__/tool-call-audit-rules.test.tsx
```
Expected: FAIL — 'Add rule' opens no dialog, no 'Edit'/'Switch to JSON' buttons exist.

- [ ] **Step 3: Implement the section changes**

Modify `web/src/features/system-settings/request-limits/tool-call-audit-section.tsx`:

3a. Imports — add after the existing lucide/UI imports (keep the existing import block style; there are no lucide imports yet, so add one):

```tsx
import { Code2, Eye } from 'lucide-react'
```

and replace the existing `./tool-call-audit-section` local imports with:

```tsx
import {
  ToolCallAuditRuleDialog,
  type ToolCallAuditRuleDraft,
} from './tool-call-audit-rule-dialog'
```

(Do NOT import `RULE_DIALOG_FORM_ID` in the section — only the tests need it, and they import it from the dialog module directly.)

3b. Delete the local `type RuleDraft = { ... }` definition (lines ~114-121) — the shared `ToolCallAuditRuleDraft` from the dialog module replaces it.

3c. Rename usages: `RuleDraft` → `ToolCallAuditRuleDraft` in `parseRules` return type, `setRules` parameter, and `visualRules` memo.

3d. State — add inside `ToolCallAuditSection` next to the existing `testResult` state:

```tsx
const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
const [ruleDialogOpen, setRuleDialogOpen] = useState(false)
const [editingRule, setEditingRule] = useState<ToolCallAuditRuleDraft | null>(
  null
)
```

3e. Handlers — add after `handleTest`:

```tsx
const handleToggleEditMode = () => {
  if (editMode === 'json') {
    try {
      const parsed = JSON.parse(rulesText || '[]')
      if (!Array.isArray(parsed)) throw new Error()
    } catch {
      toast.error(t('Rules must be a valid JSON array'))
      return
    }
  }
  setEditMode((prev) => (prev === 'visual' ? 'json' : 'visual'))
}

const openAddRule = () => {
  setEditingRule(null)
  setRuleDialogOpen(true)
}

const openEditRule = (rule: ToolCallAuditRuleDraft) => {
  setEditingRule(rule)
  setRuleDialogOpen(true)
}

const handleSaveRule = (rule: ToolCallAuditRuleDraft) => {
  const next = [...visualRules]
  const index = next.findIndex((item) => item.id === rule.id)
  if (index >= 0) {
    next[index] = rule
  } else {
    next.push(rule)
  }
  setRules(next)
  setRuleDialogOpen(false)
  setEditingRule(null)
}
```

3f. Replace the entire Rules block — from the `<div className='space-y-3'>` that starts with the `<div className='flex flex-wrap items-center justify-between gap-2'>` header (currently containing `Restore defaults` + `Add rule`) through the end of the rules `FormField` (the `Tool audit rules JSON` JsonCodeEditor) — with:

```tsx
<div className='space-y-3'>
  <div className='flex flex-wrap items-center justify-between gap-2'>
    <FormLabel>{t('Rules')}</FormLabel>
    <div className='flex gap-2'>
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={() => restoreDefaults.mutate()}
        disabled={restoreDefaults.isPending}
      >
        {t('Restore defaults')}
      </Button>
      {editMode === 'visual' && (
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={openAddRule}
        >
          {t('Add rule')}
        </Button>
      )}
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={handleToggleEditMode}
      >
        {editMode === 'visual' ? (
          <>
            <Code2 className='mr-2 h-4 w-4' />
            {t('Switch to JSON')}
          </>
        ) : (
          <>
            <Eye className='mr-2 h-4 w-4' />
            {t('Switch to Visual')}
          </>
        )}
      </Button>
    </div>
  </div>
  {editMode === 'visual' ? (
    <div className='space-y-3'>
      {visualRules.map((rule, index) => (
        <div
          key={rule.id || index}
          className='border-border flex items-center gap-3 rounded-lg border p-3'
        >
          <Switch
            aria-label={t('Enabled')}
            checked={rule.enabled !== false}
            onCheckedChange={(enabled) => {
              const next = [...visualRules]
              next[index] = { ...rule, enabled }
              setRules(next)
            }}
          />
          <div className='min-w-0 flex-1'>
            <p className='truncate text-sm font-medium'>
              {rule.name || rule.id || t('New rule')}
            </p>
            <p className='text-muted-foreground text-xs'>
              {[rule.category, rule.severity].filter(Boolean).join(' · ')}
            </p>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => openEditRule(rule)}
          >
            {t('Edit')}
          </Button>
          <Button
            type='button'
            variant='destructive'
            size='sm'
            onClick={() =>
              setRules(
                visualRules.filter((_, ruleIndex) => ruleIndex !== index)
              )
            }
          >
            {t('Delete')}
          </Button>
        </div>
      ))}
    </div>
  ) : (
    <FormField
      control={form.control}
      name='rules'
      render={({ field }) => (
        <FormItem>
          <FormLabel>{t('Tool audit rules JSON')}</FormLabel>
          <FormControl>
            <JsonCodeEditor
              value={field.value}
              onChange={field.onChange}
              name={field.name}
              onBlur={field.onBlur}
              textareaRef={field.ref}
              heightClassName='h-96 min-h-96'
            />
          </FormControl>
          <FormDescription>
            {t(
              'Rules use contains, exact, glob, regex, or keyword_set matching.'
            )}
          </FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )}
  <ToolCallAuditRuleDialog
    open={ruleDialogOpen}
    onOpenChange={setRuleDialogOpen}
    editData={editingRule}
    onSave={handleSaveRule}
  />
</div>
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `web/`):
```
bun run test --request-limits/__tests__/tool-call-audit-rules.test.tsx
```
Expected: PASS — all 4 existing + 3 new tests.

- [ ] **Step 5: Typecheck and lint**

Run (from `web/`):
```
bun run typecheck
bun run lint
```
Expected: both exit 0.

- [ ] **Step 6: Commit**

```bash
git add web/src/features/system-settings/request-limits/tool-call-audit-section.tsx web/src/features/system-settings/request-limits/__tests__/tool-call-audit-rules.test.tsx
git commit -m "feat: add visual/json mode toggle for tool call audit rules"
```

---

### Task 3: i18n keys for all locales

**Files:**
- Modify: `web/src/i18n/locales/en.json`, `web/src/i18n/locales/zh.json`, `web/src/i18n/locales/zh-TW.json`, `web/src/i18n/locales/fr.json`, `web/src/i18n/locales/ru.json`, `web/src/i18n/locales/ja.json`, `web/src/i18n/locales/vi.json`

**Interfaces:**
- Produces: translations for every new `t()` key introduced in Tasks 1-2, in all 7 locales. en.json values equal the key itself (source of truth).

- [ ] **Step 1: Load the i18n skill and add keys**

Load the `i18n-translate` skill (project skill at `.agents/skills/i18n-translate/SKILL.md`) and follow it. The keys to add (alphabetical within each locale file, matching the existing flat JSON structure):

```
Add argument path
Add pattern
Add tool name
Argument path is too long
Argument paths
Argument paths to inspect within the tool arguments.
At least one pattern is required
Auto-generated when adding a new rule.
Block SSH private key access
Configure a tool call audit rule.
Edit rule
Invalid argument path
Invalid category
Invalid match type
Invalid regex pattern
Invalid severity
Match type
Pattern cannot be empty
Pattern is too long
Patterns
Patterns matched against the tool arguments.
Rule ID
Rule ID is required
Rule ID is too long
Rule name
Rule name is too long
Rules must be a valid JSON array
Severity
Tool name is too long
Tool names
Tool names matched against this rule. Empty matches all tools.
Too many argument paths
Too many patterns
Too many tool names
```

Check for existing keys first — `Category`, `Edit`, `Add rule`, `Switch to JSON`, `Switch to Visual`, `Cancel`, `Add`, `Update`, `Delete`, `Enabled`, `Rules`, `Restore defaults`, `New rule`, `Tool audit rules JSON`, and the rules description already exist in all locales and must NOT be duplicated.

- [ ] **Step 2: Verify sync**

Run (from `web/`):
```
bun run i18n:sync
bun run test --request-limits/__tests__/tool-call-audit-rule-dialog.test.tsx --request-limits/__tests__/tool-call-audit-rules.test.tsx
```
Expected: sync exits 0 (or reports no missing keys for the new strings); both test files PASS.

- [ ] **Step 3: Commit**

```bash
git add web/src/i18n/locales/en.json web/src/i18n/locales/zh.json web/src/i18n/locales/zh-TW.json web/src/i18n/locales/fr.json web/src/i18n/locales/ru.json web/src/i18n/locales/ja.json web/src/i18n/locales/vi.json
git commit -m "feat: add i18n keys for tool call audit rule dialog"
```

---

### Task 4: Final verification

**Files:** none (verification only).

- [ ] **Step 1: Run the full check set**

Run (from `web/`):
```
bun run typecheck
bun run lint
bun run test --request-limits/__tests__/tool-call-audit-rule-dialog.test.tsx --request-limits/__tests__/tool-call-audit-rules.test.tsx
bun run format:check
```
Expected: all exit 0. If `format:check` reports header/format issues, run `bun run format` and re-run.

- [ ] **Step 2: Manual smoke check (optional but recommended)**

Start the dev server (`bun run dev`), open `/system-settings/security/sensitive-words`, and verify: default GUI mode shows the rule list; "Switch to JSON" shows the editor; entering invalid JSON then "Switch to Visual" shows a toast and stays in JSON mode; "Add rule" opens the dialog and saving appends the rule; editing pre-fills and updates the row; "Save tool audit settings" persists.

- [ ] **Step 3: No commit needed** (verification task; changes were committed per task)