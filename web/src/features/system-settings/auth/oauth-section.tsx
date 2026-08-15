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
import axios from 'axios'
import { useEffect, useMemo, useRef, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Switch } from '@/components/ui/switch'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { buildOAuthCallbackUrl, resolveOAuthSiteUrl } from './oauth-callback-url'

/**
 * react-hook-form 7 treats dotted `name` strings as nested paths. To keep
 * form state, schema validation, and dirty tracking aligned, the `oidc.*`
 * fields are modeled as nested objects here and flattened back to dotted
 * server keys only when persisting.
 */
const oauthSchema = z.object({
  oidc: z.object({
    enabled: z.boolean(),
    display_name: z.string(),
    client_id: z.string(),
    client_secret: z.string(),
    well_known: z.string(),
    authorization_endpoint: z.string(),
    token_endpoint: z.string(),
    user_info_endpoint: z.string(),
  }),
})

type OAuthFormValues = z.infer<typeof oauthSchema>

type FlatOAuthDefaults = {
  'oidc.enabled': boolean
  'oidc.display_name': string
  'oidc.client_id': string
  'oidc.client_secret': string
  'oidc.well_known': string
  'oidc.authorization_endpoint': string
  'oidc.token_endpoint': string
  'oidc.user_info_endpoint': string
}

const oauthTabContentClassName =
  'grid min-w-0 gap-x-5 gap-y-6 lg:grid-cols-2 [&>[data-slot=form-item]]:min-w-0 lg:[&>[data-slot=form-item]:has([data-slot=switch])]:col-span-2'

type OAuthSetupGuideRow = {
  label: ReactNode
  value: string
  copyLabel: string
}

type OAuthSetupGuideProps = {
  title: string
  description: ReactNode
  rows: OAuthSetupGuideRow[]
}

function OAuthSetupGuide(props: OAuthSetupGuideProps) {
  return (
    <Alert className='lg:col-span-2'>
      <AlertTitle>{props.title}</AlertTitle>
      <AlertDescription className='space-y-3 text-sm'>
        <div>{props.description}</div>
        <div className='space-y-2'>
          {props.rows.map((row) => (
            <div
              key={`${String(row.label)}-${row.value}`}
              className='flex min-w-0 flex-col gap-1.5 sm:flex-row sm:items-center sm:justify-between'
            >
              <span className='text-muted-foreground shrink-0'>
                {row.label}
              </span>
              <span className='flex min-w-0 items-center gap-2'>
                <code className='bg-muted text-foreground min-w-0 rounded px-1.5 py-0.5 text-xs break-all'>
                  {row.value}
                </code>
                <CopyButton
                  value={row.value}
                  size='icon'
                  className='size-7'
                  tooltip={row.copyLabel}
                  aria-label={row.copyLabel}
                />
              </span>
            </div>
          ))}
        </div>
      </AlertDescription>
    </Alert>
  )
}

const buildFormDefaults = (defaults: FlatOAuthDefaults): OAuthFormValues => ({
  oidc: {
    enabled: defaults['oidc.enabled'],
    display_name: defaults['oidc.display_name'] ?? '',
    client_id: defaults['oidc.client_id'] ?? '',
    client_secret: defaults['oidc.client_secret'] ?? '',
    well_known: defaults['oidc.well_known'] ?? '',
    authorization_endpoint: defaults['oidc.authorization_endpoint'] ?? '',
    token_endpoint: defaults['oidc.token_endpoint'] ?? '',
    user_info_endpoint: defaults['oidc.user_info_endpoint'] ?? '',
  },
})

const normalizeFormValues = (values: OAuthFormValues): FlatOAuthDefaults => ({
  'oidc.enabled': values.oidc.enabled,
  'oidc.display_name': values.oidc.display_name,
  'oidc.client_id': values.oidc.client_id,
  'oidc.client_secret': values.oidc.client_secret,
  'oidc.well_known': values.oidc.well_known,
  'oidc.authorization_endpoint': values.oidc.authorization_endpoint,
  'oidc.token_endpoint': values.oidc.token_endpoint,
  'oidc.user_info_endpoint': values.oidc.user_info_endpoint,
})

type OAuthSectionProps = {
  defaultValues: FlatOAuthDefaults
  serverAddress: string
}

export function OAuthSection(props: OAuthSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const siteUrl = resolveOAuthSiteUrl(props.serverAddress, t('Site URL'))
  const oidcCallbackUrl = buildOAuthCallbackUrl(
    props.serverAddress,
    'oidc',
    t('Site URL')
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<OAuthFormValues>({
    resolver: zodResolver(oauthSchema),
    defaultValues: formDefaults,
  })

  const baselineRef = useRef<FlatOAuthDefaults>(props.defaultValues)
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(props.defaultValues)
  )

  useEffect(() => {
    const serialized = JSON.stringify(props.defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = props.defaultValues
    baselineSerializedRef.current = serialized
    form.reset(buildFormDefaults(props.defaultValues))
  }, [props.defaultValues, form])

  const onSubmit = async (values: OAuthFormValues) => {
    let finalValues = values

    if (values.oidc.well_known && values.oidc.well_known.trim() !== '') {
      const wellKnown = values.oidc.well_known.trim()
      if (
        !wellKnown.startsWith('http://') &&
        !wellKnown.startsWith('https://')
      ) {
        toast.error(t('Well-Known URL must start with http:// or https://'))
        return
      }

      try {
        const res = await axios.create().get(wellKnown)
        const authEndpoint = res.data['authorization_endpoint'] || ''
        const tokenEndpoint = res.data['token_endpoint'] || ''
        const userInfoEndpoint = res.data['userinfo_endpoint'] || ''

        finalValues = {
          ...values,
          oidc: {
            ...values.oidc,
            authorization_endpoint: authEndpoint,
            token_endpoint: tokenEndpoint,
            user_info_endpoint: userInfoEndpoint,
          },
        }

        form.setValue('oidc.authorization_endpoint', authEndpoint)
        form.setValue('oidc.token_endpoint', tokenEndpoint)
        form.setValue('oidc.user_info_endpoint', userInfoEndpoint)

        toast.success(t('OIDC configuration fetched successfully'))
      } catch (err) {
        // eslint-disable-next-line no-console
        console.error(err)
        toast.error(
          t(
            'Failed to fetch OIDC configuration. Please check the URL and network status'
          )
        )
        return
      }
    }

    const normalized = normalizeFormValues(finalValues)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof FlatOAuthDefaults>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of changedKeys) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }

    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
    form.reset(buildFormDefaults(normalized))
  }

  const handleReset = () => {
    form.reset(buildFormDefaults(baselineRef.current))
    toast.success(t('Form reset to saved values'))
  }

  return (
    <>
      <FormNavigationGuard when={form.formState.isDirty} />

      <SettingsSection title={t('OAuth Integrations')}>
        <Form {...form}>
          <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
            <SettingsPageFormActions
              onSave={form.handleSubmit(onSubmit)}
              onReset={handleReset}
              isSaving={updateOption.isPending}
              isResetDisabled={!form.formState.isDirty}
            />
            <FormDirtyIndicator isDirty={form.formState.isDirty} />

            <div className={oauthTabContentClassName}>
              <OAuthSetupGuide
                title={t('Setup guide')}
                description={
                  <div className='space-y-1'>
                    <p>
                      {t(
                        'Set these values in the provider application before enabling login.'
                      )}
                    </p>
                    <p>
                      {t(
                        'OIDC discovery can fill the endpoint fields automatically when the provider supports it.'
                      )}
                    </p>
                  </div>
                }
                rows={[
                  {
                    label: t('Homepage URL'),
                    value: siteUrl,
                    copyLabel: t('Copy homepage URL'),
                  },
                  {
                    label: t('Redirect URL'),
                    value: oidcCallbackUrl,
                    copyLabel: t('Copy redirect URL'),
                  },
                ]}
              />

              <FormField
                control={form.control}
                name='oidc.enabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable OIDC')}</FormLabel>
                      <FormDescription>
                        {t('Allow users to sign in with OpenID Connect')}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              <FormField
                control={form.control}
                name='oidc.display_name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('OIDC Display Name')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. Company SSO')}
                        autoComplete='off'
                        value={field.value ?? ''}
                        onChange={(event) =>
                          field.onChange(event.target.value)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Defaults to "OIDC" if left blank')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='oidc.client_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Client ID')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('OIDC Client ID')}
                        autoComplete='off'
                        value={field.value ?? ''}
                        onChange={(event) =>
                          field.onChange(event.target.value)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='oidc.client_secret'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Client Secret')}</FormLabel>
                    <FormControl>
                      <Input
                        type='password'
                        placeholder={t('OIDC Client Secret')}
                        autoComplete='new-password'
                        value={field.value ?? ''}
                        onChange={(event) =>
                          field.onChange(event.target.value)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='oidc.well_known'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Well-Known URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t(
                          'https://provider.com/.well-known/openid-configuration'
                        )}
                        autoComplete='off'
                        value={field.value ?? ''}
                        onChange={(event) =>
                          field.onChange(event.target.value)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Auto-discovers endpoints from the provider')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='oidc.authorization_endpoint'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Authorization Endpoint (Optional)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Override auto-discovered endpoint')}
                        autoComplete='off'
                        value={field.value ?? ''}
                        onChange={(event) =>
                          field.onChange(event.target.value)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='oidc.token_endpoint'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Token Endpoint (Optional)')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Override auto-discovered endpoint')}
                        autoComplete='off'
                        value={field.value ?? ''}
                        onChange={(event) =>
                          field.onChange(event.target.value)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='oidc.user_info_endpoint'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('User Info Endpoint (Optional)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Override auto-discovered endpoint')}
                        autoComplete='off'
                        value={field.value ?? ''}
                        onChange={(event) =>
                          field.onChange(event.target.value)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
