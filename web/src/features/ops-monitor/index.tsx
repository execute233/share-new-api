import {useQuery} from '@tanstack/react-query'
import {RefreshCw} from 'lucide-react'
import {useState} from 'react'
import {useTranslation} from 'react-i18next'
import {ErrorState} from '@/components/error-state'
import {SectionPageLayout} from '@/components/layout'
import {LoadingState} from '@/components/loading-state'
import {Alert, AlertDescription} from '@/components/ui/alert'
import {Button} from '@/components/ui/button'
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card'
import {Input} from '@/components/ui/input'
import {toIntlLocale} from '@/i18n/languages'
import {type Filters, getOpsData, monitoringRange} from './api'
import {OpsDetails} from './details'
import {OpsOverview} from './overview'
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select.tsx";

export function OpsMonitor() {
    const {t, i18n} = useTranslation()
    const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
    const [period, setPeriod] = useState('1')
    const [customStart, setCustomStart] = useState('')
    const [customEnd, setCustomEnd] = useState('')
    const [draft, setDraft] = useState<Filters>({channel_id: '', model: '', group: ''})
    const [filters, setFilters] = useState(draft)
    const [paused, setPaused] = useState(false)
    const range = monitoringRange(period, customStart, customEnd)
    const query = useQuery({
        queryKey: ['ops-monitor', filters, period, customStart, customEnd],
        queryFn: ({signal}) => {
            const currentRange = monitoringRange(period, customStart, customEnd)
            if (!currentRange) throw new Error(t('Invalid time range'))
            return getOpsData(currentRange, filters, signal)
        },
        enabled: !!range,
        refetchInterval: paused ? false : 10_000,
        staleTime: 5_000,
    })
    const data = query.data
    const dateFormat = new Intl.DateTimeFormat(locale, {
        timeZone: 'Asia/Shanghai',
        dateStyle: 'short',
        timeStyle: 'medium'
    })
    return <SectionPageLayout>
        <SectionPageLayout.Title>{t('Operations monitoring')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
            <Button variant='outline'
                    onClick={() => setPaused(!paused)}>{paused ? t('Resume refresh') : t('Pause refresh')}</Button>
            <Button variant='outline' disabled={!range || query.isFetching}
                    onClick={() => void query.refetch()}><RefreshCw/>{t('Refresh')}</Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
            <div className='space-y-5'>
                <form className='flex flex-wrap items-center gap-2' onSubmit={(event) => {
                    event.preventDefault();
                    setFilters(draft)
                }}>
                    <Input className='w-36' aria-label={t('Channel ID')} placeholder={t('Channel ID')} type='number'
                           min={1} value={draft.channel_id}
                           onChange={(event) => setDraft({...draft, channel_id: event.target.value})}/>
                    <Input className='w-48' aria-label={t('Model')} placeholder={t('Model')} maxLength={256}
                           value={draft.model} onChange={(event) => setDraft({...draft, model: event.target.value})}/>
                    <Input className='w-40' aria-label={t('Group')} placeholder={t('Group')} maxLength={128}
                           value={draft.group} onChange={(event) => setDraft({...draft, group: event.target.value})}/>
                    <Button type='submit' variant='outline'>{t('Apply filters')}</Button>
                    <Select value={period} onValueChange={(value) => {
                        if (value) setPeriod(value)
                    }}>
                        <SelectTrigger aria-label={t('Time range')}>
                            <SelectValue/>
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value='1'>{t('Last hour')}</SelectItem>
                            <SelectItem value='6'>{t('Last 6 hours')}</SelectItem>
                            <SelectItem value='24'>{t('Last 24 hours')}</SelectItem>
                            <SelectItem value='168'>{t('Last 7 days')}</SelectItem>
                            <SelectItem value='720'>{t('Last 30 days')}</SelectItem>
                            <SelectItem value='custom'>{t('Custom')}</SelectItem>
                        </SelectContent>
                    </Select>
                    {period === 'custom' && <>
                        <Input className='w-auto' type='datetime-local' aria-label={t('Start time')} value={customStart}
                               onChange={(event) => setCustomStart(event.target.value)}/>
                        <Input className='w-auto' type='datetime-local' aria-label={t('End time')} value={customEnd}
                               onChange={(event) => setCustomEnd(event.target.value)}/>
                    </>}
                    <span
                        className='text-muted-foreground text-xs'>Asia/Shanghai · {t('Refresh every 10 seconds')}</span>
                </form>
                {!range && <Alert
                    variant='destructive'><AlertDescription>{t('Select a valid range within the last 30 days (UTC+8).')}</AlertDescription></Alert>}
                {query.isError && <ErrorState description={query.error.message} onRetry={() => void query.refetch()}/>}
                {range && query.isPending && <LoadingState/>}
                {range && data && <>
                    <p className='text-muted-foreground text-xs'>{t('Collection started: {{time}}', {time: dateFormat.format(data.snapshot.started_at * 1000)})} · {t('Updated at {{time}}', {time: dateFormat.format(data.snapshot.generated_at * 1000)})}</p>
                    {(data.live.write_failed || data.live.dropped > 0 || data.live.pending > 1000) && <Alert
                        variant='destructive'><AlertDescription>{t('Monitoring data may be incomplete. Pending: {{pending}}; dropped since restart: {{dropped}}.', {
                        pending: data.live.pending,
                        dropped: data.live.dropped
                    })}</AlertDescription></Alert>}
                    {data.snapshot.start < data.snapshot.started_at &&
                        <Alert><AlertDescription>{t('The selected range includes time before collection started. Historical data is not backfilled.')}</AlertDescription></Alert>}
                    <OpsOverview data={data}/>
                    <Card>
                        <CardHeader>
                            <CardTitle>{t('Request details')}</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <OpsDetails key={JSON.stringify([filters, period, customStart, customEnd])}
                                        filters={filters} range={{start: data.snapshot.start, end: data.snapshot.end}}
                                        paused={paused}/>
                        </CardContent>
                    </Card>
                </>}
            </div>
        </SectionPageLayout.Content>
    </SectionPageLayout>
}
