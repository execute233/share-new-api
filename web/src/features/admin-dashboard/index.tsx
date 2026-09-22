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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  Activity,
  Coins,
  Gauge,
  KeyRound,
  Radio,
  RefreshCw,
  Timer,
  Users,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { getLogStats } from '@/features/usage-logs/api'
import { toIntlLocale } from '@/i18n/languages'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatCompactNumber, formatNumber } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getAdminDashboard, totalTokens, type DashboardBucket } from './api'
import { DashboardCharts, ModelDistribution } from './charts'

export function AdminDashboard() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [period, setPeriod] = useState('24h')
  const [granularity, setGranularity] = useState('hour')
  const [customStart, setCustomStart] = useState('')
  const [customEnd, setCustomEnd] = useState('')
  const [ranking, setRanking] = useState(false)
  const now = Math.floor(Date.now() / 3600000) * 3600
  let start = now - 23 * 3600
  let end = now + 3600
  if (period === 'today') {
    start = Math.floor((now + 28800) / 86400) * 86400 - 28800
  }
  if (period === '7d') start = end - 7 * 86400
  if (period === '30d') start = end - 30 * 86400
  if (period === 'custom') {
    start = Date.parse(`${customStart}:00+08:00`) / 1000
    end = Date.parse(`${customEnd}:00+08:00`) / 1000
  }
  const valid =
    Number.isFinite(start) &&
    Number.isFinite(end) &&
    end > start &&
    end - start <= 31 * 86400 &&
    start % 3600 === 0 &&
    end % 3600 === 0 &&
    end <= now + 3600
  const query = useQuery({
    queryKey: ['admin-dashboard', start, end, granularity],
    queryFn: () => getAdminDashboard(start, end, granularity),
    enabled: valid,
    refetchInterval: 60_000,
    staleTime: 30_000,
  })
  const rates = useQuery({
    queryKey: ['admin-dashboard', 'rates'],
    queryFn: async () => {
      const timestamp = Math.floor(Date.now() / 1000)
      return requireServerSuccess(
        await getLogStats({
          type: 2,
          start_timestamp: timestamp - 60,
          end_timestamp: timestamp,
        })
      ).data
    },
    refetchInterval: 60_000,
    staleTime: 30_000,
  })
  const data = query.data
  const timestampFormat = new Intl.DateTimeFormat(locale, {
    timeZone: 'Asia/Shanghai',
    dateStyle: 'short',
    timeStyle: 'short',
  })
  const number = (value: number) => formatCompactNumber(value, locale)
  const tokenValue = (row: DashboardBucket) =>
    row.incomplete_requests && !totalTokens(row)
      ? '—'
      : number(totalTokens(row))
  const columns = [
    {
      id: 'name',
      header: ranking ? t('User') : t('Model'),
      cell: (row: DashboardBucket) => (
        <span
          className='block max-w-52 truncate'
          title={ranking ? row.username : row.model_name}
        >
          {ranking ? `${row.username} #${row.user_id}` : row.model_name}
        </span>
      ),
    },
    {
      id: 'requests',
      header: t('Requests'),
      cell: (row: DashboardBucket) => number(row.requests),
    },
    { id: 'tokens', header: t('Tokens'), cell: tokenValue },
    {
      id: 'quota',
      header: t('Usage charge'),
      cell: (row: DashboardBucket) => formatQuotaWithCurrency(row.quota),
    },
  ]
  const incomplete = (data?.trend ?? []).reduce(
    (sum, row) => sum + row.incomplete_requests,
    0
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Admin Dashboard')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          disabled={query.isFetching || !valid}
          onClick={() => {
            void query.refetch()
            void rates.refetch()
          }}
        >
          <RefreshCw />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-5'>
          {query.isError && (
            <ErrorState
              onRetry={() => {
                void query.refetch()
              }}
            />
          )}
          {query.isLoading && <LoadingState />}
          {data && (
            <>
              <p className='text-muted-foreground text-xs'>
                {t('Collection started: {{time}}', {
                  time: timestampFormat.format(data.started_at * 1000),
                })}{' '}
                · Asia/Shanghai ·{' '}
                {t('Updated at {{time}}', {
                  time: timestampFormat.format(
                    (data.total.updated_at || data.started_at) * 1000
                  ),
                })}
              </p>
              <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-4'>
                <StatCard
                  title={t('API Keys')}
                  value={number(data.keys)}
                  description={t('Enabled: {{count}}', {
                    count: formatNumber(data.active_keys, locale),
                  })}
                  icon={KeyRound}
                />
                <StatCard
                  title={t('Channels')}
                  value={number(data.channels)}
                  description={t('Enabled: {{count}}', {
                    count: formatNumber(data.active_channels, locale),
                  })}
                  icon={Radio}
                />
                <StatCard
                  title={t('Today requests')}
                  value={number(data.today.requests)}
                  description={t('Since collection: {{count}}', {
                    count: number(data.total.requests),
                  })}
                  icon={Activity}
                />
                <StatCard
                  title={t('Users')}
                  value={`+${number(data.new_users)}`}
                  description={t('Total: {{count}}', {
                    count: number(data.users),
                  })}
                  icon={Users}
                />
                <StatCard
                  title={t('Today tokens')}
                  value={tokenValue(data.today)}
                  description={`${t('Usage charge')}: ${formatQuotaWithCurrency(data.today.quota)}`}
                  icon={Coins}
                />
                <StatCard
                  title={t('Tokens since collection')}
                  value={tokenValue(data.total)}
                  description={`${t('Usage charge')}: ${formatQuotaWithCurrency(data.total.quota)}`}
                  icon={Coins}
                />
                <StatCard
                  title={t('Log rate (last 60 seconds)')}
                  value={rates.data ? `${number(rates.data.rpm)} RPM` : '—'}
                  description={
                    rates.data
                      ? `${number(rates.data.tpm)} TPM`
                      : t('No data available')
                  }
                  icon={Gauge}
                />
                <StatCard
                  title={t('Average response time')}
                  value={
                    data.today.timed_requests
                      ? `${formatNumber(data.today.duration_seconds / data.today.timed_requests, locale)} s`
                      : '—'
                  }
                  description={t('Active users today: {{count}}', {
                    count: number(data.active_users),
                  })}
                  icon={Timer}
                />
              </div>
              {(data.total.incomplete_requests > 0 ||
                start < data.started_at) && (
                <p className='text-muted-foreground text-sm' role='status'>
                  {t(
                    'Only usage collected after launch is included. Missing or estimated token details are marked incomplete.'
                  )}{' '}
                  {t('Incomplete requests in range: {{count}}', {
                    count: formatNumber(incomplete, locale),
                  })}
                </p>
              )}
              <Card>
                <CardHeader>
                  <CardTitle>{t('Quick actions')}</CardTitle>
                </CardHeader>
                <CardContent className='flex flex-wrap gap-3'>
                  <Button variant='outline' render={<Link to='/channels' />}>
                    {t('Channels')}
                  </Button>
                  <Button variant='outline' render={<Link to='/users' />}>
                    {t('Users')}
                  </Button>
                  <Button
                    variant='outline'
                    render={
                      <Link
                        to='/usage-logs/$section'
                        params={{ section: 'common' }}
                      />
                    }
                  >
                    {t('Usage Logs')}
                  </Button>
                </CardContent>
              </Card>
            </>
          )}
          <div className='flex flex-wrap items-center gap-3 rounded-xl border p-4'>
            <Select
              value={period}
              onValueChange={(value) => {
                if (value) setPeriod(value)
              }}
            >
              <SelectTrigger aria-label={t('Time range')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='24h'>{t('Last 24 hours')}</SelectItem>
                <SelectItem value='today'>{t('Today')}</SelectItem>
                <SelectItem value='7d'>{t('Last 7 days')}</SelectItem>
                <SelectItem value='30d'>{t('Last 30 days')}</SelectItem>
                <SelectItem value='custom'>{t('Custom')}</SelectItem>
              </SelectContent>
            </Select>
            {period === 'custom' && (
              <>
                <Input
                  aria-label={t('Start time')}
                  className='w-auto'
                  type='datetime-local'
                  step={3600}
                  value={customStart}
                  onChange={(event) => setCustomStart(event.target.value)}
                />
                <Input
                  aria-label={t('End time')}
                  className='w-auto'
                  type='datetime-local'
                  step={3600}
                  value={customEnd}
                  onChange={(event) => setCustomEnd(event.target.value)}
                />
              </>
            )}
            <span className='text-muted-foreground text-xs'>Asia/Shanghai</span>
            <Select
              value={granularity}
              onValueChange={(value) => {
                if (value) setGranularity(value)
              }}
            >
              <SelectTrigger aria-label={t('Granularity')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='hour'>{t('Hourly')}</SelectItem>
                <SelectItem value='day'>{t('Daily')}</SelectItem>
              </SelectContent>
            </Select>
            {!valid && (
              <p role='alert' className='text-destructive text-sm'>
                {t(
                  'Choose whole hours within the last available hour, up to 31 days.'
                )}
              </p>
            )}
          </div>
          {data && valid && (
            <>
              <div className='grid gap-5 xl:grid-cols-2'>
                <Card className='min-w-0'>
                  <CardHeader className='flex-row flex-wrap items-center justify-between gap-2'>
                    <CardTitle>
                      {ranking
                        ? t('User spending ranking (Top 12)')
                        : t('Model distribution (Top 20 by charge)')}
                    </CardTitle>
                    <Button
                      size='sm'
                      variant='outline'
                      onClick={() => setRanking(!ranking)}
                    >
                      {ranking ? t('Model distribution') : t('User ranking')}
                    </Button>
                  </CardHeader>
                  <CardContent>
                    {!ranking && <ModelDistribution data={data} />}
                    <StaticDataTable
                      data={(ranking ? data.ranking : data.models) ?? []}
                      columns={columns}
                      getRowKey={(row) =>
                        ranking ? row.user_id : row.model_name
                      }
                      emptyContent={t('No data available')}
                    />
                  </CardContent>
                </Card>
                <Card className='min-w-0'>
                  <CardHeader>
                    <CardTitle>{t('Token usage trend')}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <DashboardCharts
                      data={data}
                      start={start}
                      end={end}
                      daily={granularity === 'day'}
                    />
                    <p className='text-muted-foreground mt-3 text-xs'>
                      {t(
                        'Input excludes cache. Cache hit rate = cache read / all input tokens.'
                      )}
                    </p>
                  </CardContent>
                </Card>
              </div>
              <Card>
                <CardHeader>
                  <CardTitle>{t('Usage trend of top 12 spenders')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <DashboardCharts
                    data={data}
                    start={start}
                    end={end}
                    daily={granularity === 'day'}
                    users
                  />
                </CardContent>
              </Card>
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
