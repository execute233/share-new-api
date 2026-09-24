import {Activity, Cpu, Database, Gauge, MemoryStick, Radio, Timer, TriangleAlert} from 'lucide-react'
import {useTranslation} from 'react-i18next'
import {StaticDataTable} from '@/components/data-table'
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card'
import {StatCard} from '@/features/dashboard/components/ui/stat-card'
import {toIntlLocale} from '@/i18n/languages'
import {formatCompactNumber, formatNumber} from '@/lib/format'
import {type OpsData, type Percentiles, successRate} from './api'
import {OpsTrend} from './charts'

export function OpsOverview(props: { data: OpsData }) {
    const {t, i18n} = useTranslation()
    const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
    const snapshot = props.data.snapshot
    const summary = snapshot.summary
    const number = (value: number) => formatCompactNumber(value, locale)
    const percent = (value: number | null) => value == null ? '—' : `${formatNumber(value, locale)}%`
    const ms = (value: number | null) => value == null ? '—' : `${formatNumber(value, locale)} ms`
    const combined = {
        success: summary.success + snapshot.submissions.success,
        failure: summary.failure + snapshot.submissions.failure
    }
    const requests = summary.requests + snapshot.submissions.requests
    const rate = successRate(combined)
    const recent = props.data.recent
    const recentSeconds = Math.max(0, recent.end - Math.max(recent.start, recent.started_at))
    const qps = recentSeconds ? (recent.summary.requests + recent.submissions.requests) / recentSeconds : null
    const tokens = summary.tokens + snapshot.submissions.tokens
    const unknownTokens = summary.unknown_tokens + snapshot.submissions.unknown_tokens
    const details = (p: Percentiles) => [
        {label: 'P50 / P90', value: `${ms(p.p50)} / ${ms(p.p90)}`},
        {label: 'P95', value: ms(p.p95)},
        {label: t('Average'), value: ms(p.avg)},
        {label: t('Maximum'), value: ms(p.max)},
        {label: t('Samples'), value: number(p.count)},
    ]
    const channels = new Map(snapshot.channels.map((channel) => [channel.id, {...channel, in_flight: 0}]))
    for (const live of props.data.live.channels) {
        const previous = channels.get(live.id)
        channels.set(live.id, {
            id: live.id,
            name: live.name,
            requests: 0,
            success: 0,
            failure: 0,
            rejected: 0,
            cancelled: 0,
            throttled: 0,
            retries: 0,
            tokens: 0,
            unknown_tokens: 0, ...previous,
            in_flight: live.in_flight
        })
    }
    const channelRows = [...channels.values()].sort((a, b) => b.in_flight - a.in_flight || b.failure - a.failure || b.requests - a.requests)
    const system = props.data.system
    const status = (value: boolean | null | undefined) => value == null ? t('Unknown') : value ? t('Normal') : t('Unavailable')
    return <>
        <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
            <StatCard title={t('In-flight requests')} value={number(props.data.live.in_flight)}
                      description={t('Current instance')} icon={Activity} details={[
                {label: t('QPS · last complete minute'), value: qps == null ? '—' : formatNumber(qps, locale)},
                {label: t('Requests'), value: number(requests)},
                {label: t('Tokens'), value: unknownTokens && !tokens ? '—' : number(tokens)},
            ]}/>
            <StatCard title={t('Service success rate')} value={percent(rate)}
                      description={t('Excludes business rejections and client cancellations')} icon={Gauge} details={[
                {label: t('Failures'), value: number(combined.failure)},
                {label: t('Business rejection'), value: number(summary.rejected + snapshot.submissions.rejected)},
                {label: t('Cancelled'), value: number(summary.cancelled + snapshot.submissions.cancelled)},
            ]}/>
            <StatCard title={t('Request error rate')}
                      value={percent(requests ? combined.failure / requests * 100 : null)}
                      description={t('Final failures / completed client requests')} icon={TriangleAlert}/>
            <StatCard title={t('Upstream error rate')}
                      value={percent(snapshot.attempts.requests ? snapshot.attempts.failure / snapshot.attempts.requests * 100 : null)}
                      description={t('Failed attempts / all upstream attempts')} icon={Radio} details={[
                {label: '429 / 529', value: number(snapshot.attempts.throttled)},
                {label: t('Channel retries'), value: number(snapshot.attempts.retries)},
            ]}/>
            <StatCard title={t('Request duration · P99 estimate')} value={ms(summary.latency.p99)}
                      description={t('Successful relay requests only')} icon={Timer}
                      details={details(summary.latency)}/>
            <StatCard title={t('TTFT · P99 estimate')} value={ms(summary.ttft.p99)}
                      description={t('Successful streams with a measured first response')} icon={Timer}
                      details={details(summary.ttft)}/>
            <StatCard title={t('Task submissions')} value={number(snapshot.submissions.requests)}
                      description={t('Submission is separate from task completion')} icon={Activity} details={[
                {label: t('Service success rate'), value: percent(successRate(snapshot.submissions))},
                {label: t('P99 estimate'), value: ms(snapshot.submissions.latency.p99)},
            ]}/>
            <StatCard title={t('Task completions')} value={number(snapshot.tasks.requests)}
                      description={t('End-to-end task duration, including waiting')} icon={Timer} details={[
                {label: t('Service success rate'), value: percent(successRate(snapshot.tasks))},
                {label: t('Failures'), value: number(snapshot.tasks.failure)},
                {label: t('P99 estimate'), value: ms(snapshot.tasks.latency.p99)},
            ]}/>
        </div>
        {unknownTokens > 0 &&
            <p className='text-muted-foreground text-sm'>{t('Token totals exclude unknown usage; {{count}} requests have incomplete token data.', {count: unknownTokens})}</p>}
        <p className='text-muted-foreground text-xs'>{t('Percentiles are histogram estimates. No samples are shown as unavailable, not zero.')}</p>
        <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-5'>
            <StatCard title={t('Host CPU')} value={percent(system?.cpu ?? null)}
                      description={t('System resources are global and ignore traffic filters')} icon={Cpu}/>
            <StatCard title={t('Host memory')} value={percent(system?.memory ?? null)}
                      description={system ? `${number(system.memory_used / 1048576)} / ${number(system.memory_total / 1048576)} MiB` : '—'}
                      icon={MemoryStick}/>
            <StatCard title={t('Database pool')} value={status(system?.db_ok)}
                      description={system ? `${t('Active')}: ${system.db_in_use} · ${t('Idle')}: ${system.db_idle} · ${t('Maximum')}: ${system.db_max || '∞'}` : '—'}
                      icon={Database}/>
            <StatCard title={t('Redis pool')} value={system?.redis_enabled ? status(system.redis_ok) : t('Not enabled')}
                      description={system?.redis_enabled ? `${t('Total')}: ${system.redis_total} · ${t('Idle')}: ${system.redis_idle}` : '—'}
                      icon={Database}/>
            <StatCard title={t('Goroutines')} value={system ? number(system.goroutines) : '—'}
                      description={system ? `${t('Go heap')}: ${number(system.go_memory / 1048576)} MiB` : '—'}
                      icon={Activity}/>
        </div>
        <div className='grid gap-4 xl:grid-cols-3'>
            <Card>
                <CardHeader>
                    <CardTitle>{t('Channel concurrency')}</CardTitle>
                </CardHeader>
                <CardContent className='max-h-96 overflow-auto'>
                    <StaticDataTable data={channelRows} getRowKey={(row) => row.id} emptyContent={t('No data')} columns={[
                        {id: 'channel', header: t('Channel'), cell: (row) => row.name || `#${row.id}`},
                        {id: 'active', header: t('In-flight requests'), cell: (row) => number(row.in_flight)},
                        {id: 'failure', header: t('Failures'), cell: (row) => number(row.failure)},
                        {id: 'retry', header: t('Retries'), cell: (row) => number(row.retries)},
                    ]}/>
                </CardContent>
            </Card>
            <Card>
                <CardHeader>
                    <CardTitle>{t('Channel retry trend')}</CardTitle>
                </CardHeader>
                <CardContent>
                    <OpsTrend snapshot={snapshot} retries/>
                </CardContent>
            </Card>
            <Card>
                <CardHeader>
                    <CardTitle>{t('Throughput trend')}</CardTitle>
                </CardHeader>
                <CardContent>
                    <OpsTrend snapshot={snapshot}/><p className='text-muted-foreground mt-3 text-xs'>{t('QPS: completed requests/s. TPS: known tokens/s, attributed at request completion.')}</p>
                </CardContent>
            </Card>
        </div>
    </>
}
