import {useTranslation} from 'react-i18next'
import {CartesianGrid, Line, LineChart, XAxis, YAxis} from 'recharts'
import {ChartContainer, ChartTooltip, ChartTooltipContent} from '@/components/ui/chart'
import {EmptyState} from '@/components/empty-state'
import {toIntlLocale} from '@/i18n/languages'
import {formatCompactNumber} from '@/lib/format'
import type {Snapshot} from './api'

export function OpsTrend(props: { snapshot: Snapshot; retries?: boolean }) {
    const {t, i18n} = useTranslation()
    const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
    const snapshot = props.snapshot
    const config = {
        qps: {label: 'QPS', color: 'var(--chart-1)'},
        tps: {label: 'TPS', color: 'var(--chart-2)'},
        retries: {label: t('Channel retries'), color: 'var(--chart-3)'},
    }
    const points = snapshot.trend.map((point) => {
        const start = Math.max(point.bucket, snapshot.start, snapshot.started_at)
        const end = Math.min(point.bucket + snapshot.step, snapshot.end, snapshot.generated_at)
        const seconds = Math.max(0, end - start)
        return {
            time: point.bucket * 1000,
            qps: seconds ? point.requests / seconds : null,
            tps: seconds && (!point.unknown_tokens || point.tokens > 0) ? point.tokens / seconds : null,
            retries: seconds ? point.retries : null,
        }
    })
    if (!points.some((point) => point.qps !== null)) return <EmptyState title={t('No data')}/>
    const dateFormat = new Intl.DateTimeFormat(locale, {
        timeZone: 'Asia/Shanghai',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false
    })
    return <ChartContainer config={config} className='h-64 w-full'>
        <LineChart data={points} accessibilityLayer>
            <CartesianGrid vertical={false}/>
            <XAxis dataKey='time' minTickGap={40} tickFormatter={(value: number) => dateFormat.format(value)}/>
            <YAxis yAxisId='left' width={55} tickFormatter={(value: number) => formatCompactNumber(value, locale)}/>
            {!props.retries && <YAxis yAxisId='right' orientation='right' width={55}
                                      tickFormatter={(value: number) => formatCompactNumber(value, locale)}/>}
            <ChartTooltip content={<ChartTooltipContent
                labelFormatter={(_, payload) => dateFormat.format(payload[0].payload.time)}/>}/>
            {props.retries ? <Line yAxisId='left' dataKey='retries' stroke='var(--color-retries)' dot={false}
                                   isAnimationActive={false}/> : <>
                <Line yAxisId='left' dataKey='qps' stroke='var(--color-qps)' dot={false} isAnimationActive={false}/>
                <Line yAxisId='right' dataKey='tps' stroke='var(--color-tps)' dot={false} isAnimationActive={false}/>
            </>}
        </LineChart>
    </ChartContainer>
}
