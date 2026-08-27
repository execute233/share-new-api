import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, RefreshCw, Zap } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import {
  deleteProxy,
  listProxies,
  qualityCheckProxy,
  quickAddProxies,
  testProxy,
  type ProxySummary,
} from './api'
import { ProxyMutateDrawer } from './components/proxy-mutate-drawer'

const queryKey = ['proxies'] as const

function QuickAddDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [value, setValue] = useState('')
  const [result, setResult] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const submit = async () => {
    setSaving(true)
    try {
      const response = await quickAddProxies(
        value
          .split(/\r?\n/)
          .map((line) => line.trim())
          .filter(Boolean)
      )
      if (!response.success) {
        throw new Error(response.message || t('Failed to add proxies'))
      }
      setResult(JSON.stringify(response.data?.items || [], null, 2))
      props.onSaved()
    } finally {
      setSaving(false)
    }
  }
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Quick add proxies')}</DialogTitle>
        </DialogHeader>
        <Textarea
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder='每行输入一个代理，支持以下格式：
          sock5://user:pass@192.168.1.1:1080
          http://192.168.1.1:8080
          https://user:pass@proxy.example.com:443'
          rows={8}
        />
        {result && (
          <pre className='bg-muted max-h-48 overflow-auto rounded p-2 text-xs'>
            {result}
          </pre>
        )}
        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Close')}
          </Button>
          <Button
            disabled={saving || !value.trim()}
            onClick={() => void submit()}
          >
            {t('Add')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function Proxies() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('')
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [quickOpen, setQuickOpen] = useState(false)
  const [editing, setEditing] = useState<ProxySummary | null>(null)
  const query = useQuery({
    queryKey: [...queryKey, search, status],
    queryFn: () => listProxies({ search, status, p: 1, page_size: 100 }),
  })
  const proxies = useMemo(() => query.data?.data?.items || [], [query.data])
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey })
  }
  const action = async (
    proxy: ProxySummary,
    fn: (id: number) => Promise<unknown>
  ) => {
    await fn(proxy.id)
    refresh()
  }
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Proxies')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button size='sm' variant='outline' onClick={() => setQuickOpen(true)}>
          <Zap className='mr-1 h-4 w-4' />
          {t('Quick add')}
        </Button>
        <Button
          size='sm'
          onClick={() => {
            setEditing(null)
            setDrawerOpen(true)
          }}
        >
          <Plus className='mr-1 h-4 w-4' />
          {t('Add proxy')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-3'>
          <div className='flex flex-wrap gap-2'>
            <Input
              className='max-w-xs'
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t('Search proxies')}
            />
            <Select
              value={status || 'all'}
              onValueChange={(value) =>
                setStatus(value === 'all' || value == null ? '' : value)
              }
            >
              <SelectTrigger className='w-32'>
                <SelectValue placeholder={t('All statuses')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>{t('All statuses')}</SelectItem>
                <SelectItem value='active'>{t('Active')}</SelectItem>
                <SelectItem value='inactive'>{t('Inactive')}</SelectItem>
              </SelectContent>
            </Select>
            <Button variant='ghost' size='icon' onClick={refresh}>
              <RefreshCw className='h-4 w-4' />
            </Button>
          </div>
          <div className='overflow-x-auto rounded-lg border'>
            <table className='w-full text-sm'>
              <thead>
                <tr className='border-b text-left'>
                  <th className='p-3'>{t('Name')}</th>
                  <th className='p-3'>{t('Endpoint')}</th>
                  <th className='p-3'>{t('Status')}</th>
                  <th className='p-3'>{t('Quality')}</th>
                  <th className='p-3'>{t('Actions')}</th>
                </tr>
              </thead>
              <tbody>
                {proxies.map((proxy) => (
                  <tr key={proxy.id} className='border-b last:border-0'>
                    <td className='p-3 font-medium'>{proxy.name}</td>
                    <td className='p-3'>
                      {proxy.protocol}://{proxy.host}:{proxy.port}
                    </td>
                    <td className='p-3'>
                      <Badge
                        variant={
                          proxy.status === 'active' ? 'default' : 'secondary'
                        }
                      >
                        {proxy.status}
                      </Badge>
                      {proxy.credential_decrypt_failed && (
                        <div className='text-destructive text-xs'>
                          {t('Credentials need re-entry')}
                        </div>
                      )}
                    </td>
                    <td className='p-3'>
                      {proxy.quality_grade || '-'}{' '}
                      {proxy.latency_ms != null ? `${proxy.latency_ms}ms` : ''}
                      <div className='text-muted-foreground text-xs'>
                        {proxy.ip_address || t('Not checked')}
                      </div>
                    </td>
                    <td className='p-3'>
                      <div className='flex flex-wrap gap-1'>
                        <Button
                          size='sm'
                          variant='ghost'
                          onClick={() => {
                            setEditing(proxy)
                            setDrawerOpen(true)
                          }}
                        >
                          {t('Edit')}
                        </Button>
                        <Button
                          size='sm'
                          variant='ghost'
                          onClick={() => void action(proxy, testProxy)}
                        >
                          {t('Test')}
                        </Button>
                        <Button
                          size='sm'
                          variant='ghost'
                          onClick={() => void action(proxy, qualityCheckProxy)}
                        >
                          {t('Quality')}
                        </Button>
                        <Button
                          size='sm'
                          variant='ghost'
                          onClick={() => void action(proxy, deleteProxy)}
                        >
                          {t('Delete')}
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!query.isPending && proxies.length === 0 && (
              <div className='text-muted-foreground p-8 text-center'>
                {t('No proxies')}
              </div>
            )}
          </div>
        </div>
        <ProxyMutateDrawer
          proxy={editing}
          open={drawerOpen}
          onOpenChange={setDrawerOpen}
          onSaved={refresh}
        />
        <QuickAddDialog
          open={quickOpen}
          onOpenChange={setQuickOpen}
          onSaved={refresh}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
