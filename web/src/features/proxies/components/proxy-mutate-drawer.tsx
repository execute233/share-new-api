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
import { useEffect, useState } from 'react'
import { type SubmitHandler, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

import { createProxy, updateProxy, type ProxySummary } from '../api'
import {
  PROXY_FORM_DEFAULT_VALUES,
  proxyFormSchema,
  transformProxyToFormValues,
  type ProxyFormValues,
} from '../lib/proxy-form'

export function ProxyMutateDrawer(props: {
  proxy: ProxySummary | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [saving, setSaving] = useState(false)

  const form = useForm<ProxyFormValues>({
    resolver: zodResolver(proxyFormSchema),
    defaultValues: props.proxy
      ? transformProxyToFormValues(props.proxy)
      : PROXY_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(
      props.proxy
        ? transformProxyToFormValues(props.proxy)
        : PROXY_FORM_DEFAULT_VALUES
    )
  }, [props.open, props.proxy, form])

  const isShadowsocks = form.watch('protocol') === 'ss'
  let usernamePlaceholder = ''
  if (isShadowsocks) {
    usernamePlaceholder = 'chacha20-ietf-poly1305'
  } else if (props.proxy?.credential_configured) {
    usernamePlaceholder = t('Leave empty to keep current')
  }

  const onSubmit: SubmitHandler<ProxyFormValues> = async (values) => {
    setSaving(true)
    try {
      const payload = {
        name: values.name.trim(),
        protocol: values.protocol,
        host: values.host.trim(),
        port: values.port,
        username: values.username || undefined,
        password: values.password || undefined,
        status: values.status,
      }
      const result = props.proxy
        ? await updateProxy(props.proxy.id, payload)
        : await createProxy(payload)
      if (!result.success) {
        throw new Error(result.message || t('Failed to save proxy'))
      }
      props.onSaved()
      props.onOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to save proxy')
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-lg')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {props.proxy ? t('Edit proxy') : t('Add proxy')}
          </SheetTitle>
        </SheetHeader>
        <Form {...form}>
          <form
            id='proxy-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='protocol'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Protocol')}</FormLabel>
                  <FormControl>
                    <Select
                      value={field.value}
                      onValueChange={(value) => value && field.onChange(value)}
                    >
                      <SelectTrigger className='w-full min-w-0'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {proxyFormSchema.shape.protocol.options.map(
                          (protocol) => (
                            <SelectItem key={protocol} value={protocol}>
                              {protocol}
                            </SelectItem>
                          )
                        )}
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='host'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Host')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='port'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Port')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value}
                      onChange={(e) => field.onChange(Number(e.target.value))}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='username'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {isShadowsocks ? t('Encryption method') : t('Username')}
                  </FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={usernamePlaceholder} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='password'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Password')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      {...field}
                      placeholder={
                        props.proxy?.credential_configured
                          ? t('Leave empty to keep current')
                          : ''
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            {isShadowsocks && (
              <p className='text-muted-foreground text-sm'>
                {t(
                  'For Shadowsocks, the encryption method goes in the first field (e.g. aes-256-gcm, chacha20-ietf-poly1305) and the password in the second. You can also paste a full ss:// URL in quick add.'
                )}
              </p>
            )}
            <FormField
              control={form.control}
              name='status'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Status')}</FormLabel>
                  <FormControl>
                    <Select
                      value={field.value}
                      onValueChange={(value) => value && field.onChange(value)}
                    >
                      <SelectTrigger className='w-full min-w-0'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='active'>{t('Active')}</SelectItem>
                        <SelectItem value='inactive'>
                          {t('Inactive')}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button type='submit' form='proxy-form' disabled={saving}>
            {t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
