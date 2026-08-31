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
import { useMutation } from '@tanstack/react-query'
import { Code2, Eye } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { JsonCodeEditor } from '@/components/json-code-editor'
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

import { getToolCallAuditDefaults, testToolCallAudit } from '../api'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  ToolCallAuditRuleDialog,
  type ToolCallAuditRuleDraft,
} from './tool-call-audit-rule-dialog'

const DEFAULT_CONFIG = {
  version: 1,
  mode: 'disabled',
  channel_ids: [],
  rules: [],
  max_argument_bytes: 65536,
  max_total_argument_bytes: 262144,
  max_json_depth: 32,
  log_retention_days: 7,
}

const POSITIVE_INTEGER_PATTERN = /^\d+$/

function isChannelIdList(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return true

  return trimmed.split(',').every((item) => {
    const token = item.trim()
    if (!POSITIVE_INTEGER_PATTERN.test(token)) return false

    const channelId = Number(token)
    return Number.isSafeInteger(channelId) && channelId > 0
  })
}

const schema = z
  .object({
    mode: z.enum(['disabled', 'audit', 'block']),
    channelIds: z
      .string()
      .refine(
        isChannelIdList,
        'Channel IDs must be comma-separated positive integers'
      ),
    maxArgumentBytes: z.coerce.number().int().min(1),
    maxTotalArgumentBytes: z.coerce.number().int().min(1),
    maxJsonDepth: z.coerce.number().int().min(1),
    logRetentionDays: z.coerce.number().int().min(1),
    rules: z.string().refine((value) => {
      try {
        return Array.isArray(JSON.parse(value || '[]'))
      } catch {
        return false
      }
    }, 'Rules must be a JSON array'),
  })
  .refine((values) => values.maxTotalArgumentBytes >= values.maxArgumentBytes, {
    path: ['maxTotalArgumentBytes'],
    message: 'Total argument limit must not be smaller than the per-call limit',
  })

type FormValues = z.output<typeof schema>
type FormInput = z.input<typeof schema>

type ToolCallAuditSectionProps = {
  defaultValues: string
}

function parseConfig(value: string) {
  try {
    const parsed = JSON.parse(value || '{}')
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return DEFAULT_CONFIG
    }
    return {
      ...DEFAULT_CONFIG,
      ...parsed,
      channel_ids: Array.isArray(parsed.channel_ids) ? parsed.channel_ids : [],
      rules: Array.isArray(parsed.rules) ? parsed.rules : [],
    }
  } catch {
    return DEFAULT_CONFIG
  }
}

function buildFormValues(value: string): FormInput {
  const config = parseConfig(value)
  return {
    mode:
      config.mode === 'audit' || config.mode === 'block'
        ? config.mode
        : 'disabled',
    channelIds: config.channel_ids.join(','),
    maxArgumentBytes: config.max_argument_bytes,
    maxTotalArgumentBytes: config.max_total_argument_bytes,
    maxJsonDepth: config.max_json_depth,
    logRetentionDays: config.log_retention_days,
    rules: JSON.stringify(config.rules, null, 2),
  }
}

function parseChannelIds(value: string) {
  if (!value.trim()) return []
  return value.split(',').map((item) => Number(item.trim()))
}

function parseRules(value: string): ToolCallAuditRuleDraft[] {
  try {
    const parsed = JSON.parse(value || '[]')
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function buildConfig(values: FormValues) {
  return {
    version: 1,
    mode: values.mode,
    channel_ids: parseChannelIds(values.channelIds),
    rules: parseRules(values.rules),
    max_argument_bytes: values.maxArgumentBytes,
    max_total_argument_bytes: values.maxTotalArgumentBytes,
    max_json_depth: values.maxJsonDepth,
    log_retention_days: values.logRetentionDays,
  }
}

export function ToolCallAuditSection({
  defaultValues,
}: ToolCallAuditSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const formDefaults = useMemo(
    () => buildFormValues(defaultValues),
    [defaultValues]
  )
  const form = useForm<FormInput, unknown, FormValues>({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })
  const [testToolName, setTestToolName] = useState('shell')
  const [testArguments, setTestArguments] = useState(
    '{"cmd":"cat ~/.ssh/id_rsa"}'
  )
  const [testResult, setTestResult] = useState<string>('')
  const [isTesting, setIsTesting] = useState(false)
  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
  const [ruleDialogOpen, setRuleDialogOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<ToolCallAuditRuleDraft | null>(
    null
  )
  const rulesText = form.watch('rules')
  const visualRules = useMemo(() => parseRules(rulesText), [rulesText])

  const setRules = (rules: ToolCallAuditRuleDraft[]) => {
    form.setValue('rules', JSON.stringify(rules, null, 2), {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  const restoreDefaults = useMutation({
    mutationFn: getToolCallAuditDefaults,
    onSuccess: (response) => {
      if (!response.success || !Array.isArray(response.data?.rules)) {
        toast.error(response.message || t('Request failed'))
        return
      }
      setRules(response.data.rules)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Request failed'))
    },
  })

  useEffect(() => {
    form.reset(buildFormValues(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (values: FormValues) => {
    await updateOption.mutateAsync({
      key: 'ToolCallAuditSettings',
      value: JSON.stringify(buildConfig(values)),
    })
  }

  const handleTest = form.handleSubmit(async (values) => {
    setIsTesting(true)
    setTestResult('')
    try {
      const response = await testToolCallAudit({
        settings: buildConfig(values),
        tool_name: testToolName,
        arguments: testArguments,
      })
      if (!response.success) {
        setTestResult(response.message)
        return
      }
      const ruleNames = response.data.matches
        .map((match) => match.name || match.rule_id)
        .join(', ')
      setTestResult(
        response.data.matched
          ? t('Matched rules: {{rules}}', { rules: ruleNames })
          : t('No rules matched')
      )
    } catch (error) {
      setTestResult(error instanceof Error ? error.message : t('Test failed'))
    } finally {
      setIsTesting(false)
    }
  })

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

  return (
    <SettingsSection title={t('Upstream Tool Call Audit')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save tool audit settings'
          />

          <FormField
            control={form.control}
            name='mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Audit mode')}</FormLabel>
                <Select
                  items={[
                    { value: 'disabled', label: t('Disabled') },
                    { value: 'audit', label: t('Audit only') },
                    { value: 'block', label: t('Block matched calls') },
                  ]}
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
                      <SelectItem value='disabled'>{t('Disabled')}</SelectItem>
                      <SelectItem value='audit'>{t('Audit only')}</SelectItem>
                      <SelectItem value='block'>
                        {t('Block matched calls')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Audit mode evaluates and allows matches without saving logs; block mode rejects matches and records rejection logs.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='channelIds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Channel IDs')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('Empty means all channels')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Comma-separated channel IDs. Leave empty to audit all channels.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='maxArgumentBytes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max argument bytes')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      value={field.value as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                      onBlur={field.onBlur}
                      name={field.name}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='maxTotalArgumentBytes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max total argument bytes')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      value={field.value as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                      onBlur={field.onBlur}
                      name={field.name}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='maxJsonDepth'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max JSON depth')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      value={field.value as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                      onBlur={field.onBlur}
                      name={field.name}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='logRetentionDays'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Audit log retention days')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      value={field.value as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                      onBlur={field.onBlur}
                      name={field.name}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

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
                        {[rule.category, rule.severity]
                          .filter(Boolean)
                          .join(' · ')}
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
                          visualRules.filter(
                            (_, ruleIndex) => ruleIndex !== index
                          )
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

          <div className='space-y-3 rounded-lg border p-4'>
            <FormLabel>{t('Test rules')}</FormLabel>
            <div className='grid gap-3 md:grid-cols-2'>
              <Input
                aria-label={t('Tool name')}
                value={testToolName}
                onChange={(event) => setTestToolName(event.target.value)}
                placeholder={t('Tool name')}
              />
              <Input
                aria-label={t('Tool arguments')}
                value={testArguments}
                onChange={(event) => setTestArguments(event.target.value)}
                placeholder={t('Tool arguments')}
              />
            </div>
            <div className='flex items-center gap-3'>
              <Button
                type='button'
                variant='outline'
                onClick={handleTest}
                disabled={isTesting}
              >
                {isTesting ? t('Testing...') : t('Test')}
              </Button>
              {testResult && (
                <p className='text-muted-foreground text-sm'>{testResult}</p>
              )}
            </div>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
