import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { StaticDataTable } from '@/components/data-table'
import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { getOpsEvents, type Filters, type OpsEvent, type Range } from './api'

export function OpsDetails(props: { filters: Filters; range: Range; paused: boolean }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [kind, setKind] = useState('')
  const [outcome, setOutcome] = useState('')
  const [search, setSearch] = useState({ user_id: '', request_id: '' })
  const [applied, setApplied] = useState(search)
  const [cursors, setCursors] = useState<{ before: number; before_id: string }[]>([])
  const [selected, setSelected] = useState<OpsEvent | null>(null)
  const cursor = cursors.at(-1)
  const query = useQuery({
    queryKey: ['ops-monitor-events', props.filters, props.range, kind, outcome, applied, cursor],
    queryFn: ({ signal }) => getOpsEvents({ ...props.filters, ...props.range, kind, outcome, ...applied, ...cursor }, signal),
    refetchInterval: props.paused || cursor ? false : 10_000,
  })
  const kinds: Record<string, string> = { request: t('Relay request'), attempt: t('Upstream attempt'), task_submission: t('Task submission'), task_completion: t('Task completion') }
  const outcomes: Record<string, string> = { success: t('Success'), failure: t('Failure'), rejected: t('Business rejection'), cancelled: t('Cancelled') }
  const dateFormat = new Intl.DateTimeFormat(locale, { timeZone: 'Asia/Shanghai', dateStyle: 'short', timeStyle: 'medium' })
  const rows = query.data?.items ?? []
  return <div className='space-y-4'>
    <p className='text-muted-foreground text-sm'>{t('Request and attempt details are retained for 7 days. Error text is not redacted.')}</p>
    <form className='flex flex-wrap items-center gap-2' onSubmit={(event) => { event.preventDefault(); setApplied(search); setCursors([]) }}>
      <NativeSelect aria-label={t('Event type')} value={kind} onChange={(event) => { setKind(event.target.value); setCursors([]) }}>
        <NativeSelectOption value=''>{t('All types')}</NativeSelectOption>
        {Object.entries(kinds).map(([value, label]) => <NativeSelectOption key={value} value={value}>{label}</NativeSelectOption>)}
      </NativeSelect>
      <NativeSelect aria-label={t('Status')} value={outcome} onChange={(event) => { setOutcome(event.target.value); setCursors([]) }}>
        <NativeSelectOption value=''>{t('All statuses')}</NativeSelectOption>
        {Object.entries(outcomes).map(([value, label]) => <NativeSelectOption key={value} value={value}>{label}</NativeSelectOption>)}
      </NativeSelect>
      <Input className='w-32' aria-label={t('User ID')} placeholder={t('User ID')} type='number' min={1} value={search.user_id} onChange={(event) => setSearch({ ...search, user_id: event.target.value })} />
      <Input className='w-60' aria-label={t('Request ID')} placeholder={t('Request ID')} maxLength={128} value={search.request_id} onChange={(event) => setSearch({ ...search, request_id: event.target.value })} />
      <Button type='submit' variant='outline'>{t('Search')}</Button>
    </form>
    {query.isPending && <LoadingState />}
    {query.isError && <ErrorState onRetry={() => void query.refetch()} />}
    {!query.isPending && !query.isError && <>
      <StaticDataTable data={rows} getRowKey={(row) => row.id} emptyContent={t('No data')} columns={[
        { id: 'time', header: t('Time'), cell: (row) => dateFormat.format(row.timestamp * 1000) },
        { id: 'kind', header: t('Event type'), cell: (row) => kinds[row.kind] ?? row.kind },
        { id: 'user', header: t('User'), cell: (row) => row.username || `#${row.user_id}` },
        { id: 'channel', header: t('Channel'), cell: (row) => row.channel_name || (row.channel_id ? `#${row.channel_id}` : '—') },
        { id: 'model', header: t('Model'), cell: (row) => row.model_name || '—' },
        { id: 'outcome', header: t('Status'), cell: (row) => <span className={row.outcome === 'failure' ? 'text-destructive' : undefined}>{outcomes[row.outcome]} · {row.status || '—'}</span> },
        { id: 'duration', header: t('Duration'), cell: (row) => `${formatNumber(row.duration_ms, locale)} ms` },
        { id: 'detail', header: t('Details'), cell: (row) => <Button variant='ghost' size='sm' onClick={() => setSelected(row)}>{t('View')}</Button> },
      ]} />
      <div className='flex justify-end gap-2'>
        <Button variant='outline' disabled={!cursors.length} onClick={() => setCursors(cursors.slice(0, -1))}>{t('Previous')}</Button>
        <Button variant='outline' disabled={!query.data?.has_more || !rows.length} onClick={() => { const last = rows.at(-1); if (last) setCursors([...cursors, { before: last.timestamp, before_id: last.id }]) }}>{t('Next')}</Button>
      </div>
    </>}
    <Dialog open={!!selected} onOpenChange={(open) => { if (!open) setSelected(null) }} title={t('Request details')}>
      {selected && <div className='space-y-4'>
        <div className='flex min-w-0 items-center gap-2'><span className='break-all'>{selected.request_id}</span><CopyButton value={selected.request_id} /></div>
        <dl className='grid grid-cols-2 gap-2 text-sm'>
          <dt>{t('User')}</dt><dd className='break-all'>{selected.username} (#{selected.user_id})</dd>
          <dt>{t('Channel')}</dt><dd className='break-all'>{selected.channel_name} (#{selected.channel_id})</dd>
          <dt>{t('Model')}</dt><dd className='break-all'>{selected.model_name || '—'}</dd>
          <dt>{t('Group')}</dt><dd className='break-all'>{selected.group_name || '—'}</dd>
          <dt>{t('Event type')}</dt><dd>{kinds[selected.kind]}</dd>
          <dt>{t('Status code')}</dt><dd>{selected.status || '—'}</dd>
          <dt>{t('Attempt')}</dt><dd>{selected.attempt || '—'}</dd>
          <dt>TTFT</dt><dd>{selected.ttft_ms == null ? '—' : `${formatNumber(selected.ttft_ms, locale)} ms`}</dd>
        </dl>
        <div className='flex items-center justify-between'><h3 className='font-medium'>{t('Error message')}</h3>{selected.error_message && <CopyButton value={selected.error_message} />}</div>
        {selected.error_truncated && <p className='text-warning text-sm'>{t('Error text truncated at 16 KiB.')}</p>}
        <pre className='bg-muted max-h-96 overflow-auto rounded-md p-3 text-xs break-all whitespace-pre-wrap'>{selected.error_message || t('No error')}</pre>
      </div>}
    </Dialog>
  </div>
}
