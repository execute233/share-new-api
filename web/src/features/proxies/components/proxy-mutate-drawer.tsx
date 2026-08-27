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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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

const protocols = ['http', 'https', 'socks5', 'socks5h']

type FormState = {
  name: string
  protocol: string
  host: string
  port: string
  username: string
  password: string
  status: string
}
const emptyForm: FormState = {
  name: '',
  protocol: 'http',
  host: '',
  port: '8080',
  username: '',
  password: '',
  status: 'active',
}

export function ProxyMutateDrawer(props: {
  proxy: ProxySummary | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [form, setForm] = useState<FormState>(() =>
    props.proxy
      ? {
          name: props.proxy.name,
          protocol: props.proxy.protocol,
          host: props.proxy.host,
          port: String(props.proxy.port),
          username: '',
          password: '',
          status: props.proxy.status,
        }
      : emptyForm
  )
  const [saving, setSaving] = useState(false)
  const update = (key: keyof FormState, value: string) =>
    setForm((current) => ({ ...current, [key]: value }))
  const submit = async () => {
    setSaving(true)
    try {
      const payload = {
        name: form.name.trim(),
        protocol: form.protocol,
        host: form.host.trim(),
        port: Number(form.port),
        username: form.username || undefined,
        password: form.password || undefined,
        status: form.status,
      }
      const result = props.proxy
        ? await updateProxy(props.proxy.id, payload)
        : await createProxy(payload)
      if (!result.success) {
        throw new Error(result.message || t('Failed to save proxy'))
      }
      props.onSaved()
      props.onOpenChange(false)
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
        <div className={sideDrawerFormClassName()}>
          <div className='space-y-2'>
            <Label htmlFor='proxy-name'>{t('Name')}</Label>
            <Input
              id='proxy-name'
              value={form.name}
              onChange={(e) => update('name', e.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='proxy-protocol'>{t('Protocol')}</Label>
            <Select
              value={form.protocol}
              onValueChange={(value) => value && update('protocol', value)}
            >
              <SelectTrigger id='proxy-protocol' className='w-full min-w-0'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {protocols.map((protocol) => (
                  <SelectItem key={protocol} value={protocol}>
                    {protocol}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='proxy-host'>{t('Host')}</Label>
            <Input
              id='proxy-host'
              value={form.host}
              onChange={(e) => update('host', e.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='proxy-port'>{t('Port')}</Label>
            <Input
              id='proxy-port'
              type='number'
              value={form.port}
              onChange={(e) => update('port', e.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='proxy-username'>{t('Username')}</Label>
            <Input
              id='proxy-username'
              value={form.username}
              onChange={(e) => update('username', e.target.value)}
              placeholder={
                props.proxy?.credential_configured
                  ? t('Leave empty to keep current')
                  : ''
              }
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='proxy-password'>{t('Password')}</Label>
            <Input
              id='proxy-password'
              type='password'
              value={form.password}
              onChange={(e) => update('password', e.target.value)}
              placeholder={
                props.proxy?.credential_configured
                  ? t('Leave empty to keep current')
                  : ''
              }
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='proxy-status'>{t('Status')}</Label>
            <Select
              value={form.status}
              onValueChange={(value) => value && update('status', value)}
            >
              <SelectTrigger id='proxy-status' className='w-full min-w-0'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='active'>{t('Active')}</SelectItem>
                <SelectItem value='inactive'>{t('Inactive')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={saving} onClick={() => void submit()}>
            {t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
