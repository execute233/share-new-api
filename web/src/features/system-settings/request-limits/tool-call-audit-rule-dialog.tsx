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
    const hasInvalidRegex = values.patterns.some((pattern) => {
      try {
        new RegExp(pattern)
        return false
      } catch {
        return true
      }
    })
    if (hasInvalidRegex) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['patterns'],
        message: 'Invalid regex pattern',
      })
    }
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
            patterns: Array.isArray(editData.patterns) ? editData.patterns : [],
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
          onSubmit={(event) => {
            event.stopPropagation()
            void form.handleSubmit(handleSubmit)(event)
          }}
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
                  <Input
                    {...field}
                    placeholder={t('Block SSH private key access')}
                  />
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
                  {t(
                    'Tool names matched against this rule. Empty matches all tools.'
                  )}
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
