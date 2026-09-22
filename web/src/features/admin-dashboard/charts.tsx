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
import { useTranslation } from 'react-i18next'
import {
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { EmptyState } from '@/components/empty-state'
import { ChartContainer, ChartTooltipContent } from '@/components/ui/chart'
import { toIntlLocale } from '@/i18n/languages'
import { formatCompactNumber } from '@/lib/format'

import { totalTokens, type DashboardSnapshot } from './api'

const colors = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
]

export function DashboardCharts(props: {
  data: DashboardSnapshot
  start: number
  end: number
  daily: boolean
  users?: boolean
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const labels = {
    input_tokens: t('Uncached input'),
    output_tokens: t('Output'),
    cache_creation_tokens: t('Cache creation'),
    cache_read_tokens: t('Cache read'),
    hit_rate: t('Cache hit rate'),
  }
  const config = Object.fromEntries(
    Object.entries(labels).map(([key, label], index) => [
      key,
      { label, color: colors[index] },
    ])
  )
  const rankings = props.data.ranking ?? []
  const userRows = props.data.users_trend ?? []
  const rows = props.data.trend ?? []
  const byBucket = new Map(rows.map((row) => [row.bucket, row]))
  const step = props.daily ? 86400 : 3600
  let first = props.start
  if (props.daily) first = Math.floor((first + 28800) / 86400) * 86400 - 28800
  const points: Record<string, number | string | null>[] = []
  const byUserBucket = new Map(
    userRows.map((row) => [`${row.bucket}:${row.user_id}`, row])
  )
  for (let bucket = first; bucket < props.end; bucket += step) {
    const row = byBucket.get(bucket)
    const collected = bucket + step > props.data.started_at
    const unknown =
      !collected || (!!row?.incomplete_requests && totalTokens(row) === 0)
    const point: Record<string, number | string | null> = {
      time: new Intl.DateTimeFormat(locale, {
        timeZone: 'Asia/Shanghai',
        month: '2-digit',
        day: '2-digit',
        ...(props.daily ? {} : { hour: '2-digit' as const, hour12: false }),
      }).format(bucket * 1000),
    }
    for (const key of [
      'input_tokens',
      'output_tokens',
      'cache_creation_tokens',
      'cache_read_tokens',
    ] as const) {
      point[key] = unknown ? null : (row?.[key] ?? 0)
    }
    const input =
      (row?.input_tokens ?? 0) +
      (row?.cache_creation_tokens ?? 0) +
      (row?.cache_read_tokens ?? 0)
    point.hit_rate =
      unknown || !input ? null : ((row?.cache_read_tokens ?? 0) / input) * 100
    for (const user of rankings) {
      const usage = byUserBucket.get(`${bucket}:${user.user_id}`)
      let value: number | null = usage ? totalTokens(usage) : 0
      if (!collected || (usage?.incomplete_requests && value === 0)) {
        value = null
      }
      point[`user_${user.user_id}`] = value
    }
    points.push(point)
  }
  const userConfig = Object.fromEntries(
    rankings.map((row, index) => [
      `user_${row.user_id}`,
      {
        label: `${row.username} #${row.user_id}`,
        color: colors[index % colors.length],
      },
    ])
  )
  if (!rows.length) return <EmptyState title={t('No data available')} />
  return (
    <ChartContainer
      config={props.users ? userConfig : config}
      className='aspect-auto h-72 w-full'
    >
      <LineChart
        data={points}
        margin={{ top: 8, right: 12, left: 0, bottom: 0 }}
      >
        <CartesianGrid vertical={false} />
        <XAxis
          dataKey='time'
          minTickGap={36}
          tickLine={false}
          axisLine={false}
        />
        <YAxis
          yAxisId='tokens'
          width={55}
          tickFormatter={(value: number) => formatCompactNumber(value, locale)}
        />
        {!props.users && (
          <YAxis
            yAxisId='rate'
            orientation='right'
            domain={[0, 100]}
            unit='%'
            width={42}
          />
        )}
        <Tooltip content={<ChartTooltipContent />} />
        <Legend />
        {props.users
          ? rankings.map((row, index) => (
              <Line
                key={row.user_id}
                yAxisId='tokens'
                type='monotone'
                dataKey={`user_${row.user_id}`}
                name={`${row.username} #${row.user_id}`}
                stroke={colors[index % colors.length]}
                strokeDasharray={index >= 5 ? '5 3' : undefined}
                dot={false}
                isAnimationActive={false}
              />
            ))
          : Object.entries(labels).map(([key, label], index) => (
              <Line
                key={key}
                yAxisId={key === 'hit_rate' ? 'rate' : 'tokens'}
                dataKey={key}
                name={label}
                stroke={colors[index]}
                strokeDasharray={key === 'hit_rate' ? '4 4' : undefined}
                dot={false}
                isAnimationActive={false}
              />
            ))}
      </LineChart>
    </ChartContainer>
  )
}

export function ModelDistribution(props: { data: DashboardSnapshot }) {
  const { t } = useTranslation()
  const rows = (props.data.models ?? []).map((row, index) => ({
    name: row.model_name,
    value: totalTokens(row),
    fill: colors[index % colors.length],
  }))
  if (!rows.some((row) => row.value > 0)) {
    return <EmptyState title={t('No data available')} />
  }
  return (
    <ChartContainer
      config={{ value: { label: t('Tokens') } }}
      className='aspect-auto h-52 w-full'
    >
      <PieChart>
        <Pie
          data={rows}
          dataKey='value'
          nameKey='name'
          innerRadius={55}
          outerRadius={85}
          isAnimationActive={false}
        >
          {rows.map((row) => (
            <Cell key={row.name} fill={row.fill} />
          ))}
        </Pie>
        <Tooltip content={<ChartTooltipContent />} />
      </PieChart>
    </ChartContainer>
  )
}
