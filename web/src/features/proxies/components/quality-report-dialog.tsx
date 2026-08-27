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

For commercial licensing, please contact support@quantumnous.com.
*/
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { formatTimestamp } from '@/lib/format'

import type { ProxyQualityItem, ProxySummary } from '../api'

const targetLabelKeys: Record<string, string> = {
  base_connectivity: 'Base connectivity',
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
  grok: 'Grok',
}

const statusVariantMap: Record<string, 'success' | 'warning' | 'danger'> = {
  pass: 'success',
  warn: 'warning',
  challenge: 'danger',
  fail: 'danger',
}

const statusLabelKeys: Record<string, string> = {
  pass: 'Pass',
  warn: 'Warn',
  challenge: 'Challenge',
  fail: 'Fail',
}

export function QualityReportDialog(props: {
  proxy: ProxySummary | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  if (!props.proxy) {
    return null
  }
  const proxy = props.proxy
  const items = proxy.quality_items ?? []
  const targetLabel = (item: ProxyQualityItem) =>
    t(targetLabelKeys[item.target] ?? item.target)
  const statusLabel = (item: ProxyQualityItem) =>
    t(statusLabelKeys[item.status] ?? item.status)
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Quality Report')}
      footer={
        <Button variant='outline' onClick={() => props.onOpenChange(false)}>
          {t('Close')}
        </Button>
      }
    >
      <div className='bg-muted/40 rounded-lg border p-4'>
        <div className='flex items-center justify-between gap-4'>
          <div className='min-w-0'>
            <div className='text-muted-foreground text-sm'>{proxy.name}</div>
            <div className='mt-1 text-sm'>{proxy.quality_summary || '-'}</div>
          </div>
          <div className='text-right'>
            <div className='text-2xl font-semibold'>
              {proxy.quality_score ?? '-'}
            </div>
            <div className='text-muted-foreground text-xs'>
              {t('Grade')}: {proxy.quality_grade || '-'}
            </div>
          </div>
        </div>
        <div className='text-muted-foreground mt-3 grid grid-cols-2 gap-2 text-xs'>
          <div>
            {t('Exit IP')}: {proxy.ip_address || '-'}
          </div>
          <div>
            {t('Country')}: {proxy.country || '-'}
          </div>
          <div>
            {t('Latency')}:{' '}
            {proxy.latency_ms != null ? `${proxy.latency_ms}ms` : '-'}
          </div>
          <div>
            {t('Checked at')}:{' '}
            {proxy.last_checked_at != null
              ? formatTimestamp(proxy.last_checked_at)
              : '-'}
          </div>
        </div>
      </div>
      <div className='overflow-x-auto rounded-lg border'>
        <table className='w-full text-sm'>
          <thead>
            <tr className='border-b text-left'>
              <th className='p-3'>{t('Target')}</th>
              <th className='p-3'>{t('Status')}</th>
              <th className='p-3'>HTTP</th>
              <th className='p-3'>{t('Latency')}</th>
              <th className='p-3'>{t('Message')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.target} className='border-b last:border-0'>
                <td className='p-3'>
                  <div className='font-medium'>{targetLabel(item)}</div>
                  {item.url ? (
                    <div className='text-muted-foreground break-all text-xs'>
                      {item.url}
                    </div>
                  ) : null}
                </td>
                <td className='p-3'>
                  <StatusBadge
                    variant={statusVariantMap[item.status] ?? 'neutral'}
                    copyable={false}
                  >
                    {statusLabel(item)}
                  </StatusBadge>
                </td>
                <td className='p-3'>{item.http_status ?? '-'}</td>
                <td className='p-3'>
                  {item.latency_ms != null ? `${item.latency_ms}ms` : '-'}
                </td>
                <td className='p-3'>
                  <span>{item.message || '-'}</span>
                  {item.cf_ray ? (
                    <span className='text-muted-foreground ml-1 text-xs'>
                      (cf-ray: {item.cf_ray})
                    </span>
                  ) : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {items.length === 0 && (
          <div className='text-muted-foreground p-8 text-center'>
            {t('No quality results')}
          </div>
        )}
      </div>
    </Dialog>
  )
}