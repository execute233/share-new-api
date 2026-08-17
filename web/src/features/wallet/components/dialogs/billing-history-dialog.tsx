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
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Skeleton } from '@/components/ui/skeleton'

import { useBillingHistory } from '../../hooks/use-billing-history'

interface BillingHistoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function BillingHistoryDialog({
  open,
  onOpenChange,
}: BillingHistoryDialogProps) {
  const { t } = useTranslation()
  const { records, total, page, pageSize, loading, handlePageChange, handlePageSizeChange } =
    useBillingHistory()
  const totalPages = Math.ceil(total / pageSize)

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Redemption History')}
      description={t('View your past redemptions and admin credits')}
      contentClassName='flex max-h-[calc(100dvh-2rem)] flex-col max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-2xl'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      <div className='min-h-0 space-y-3'>
        <div className='max-h-[min(54vh,520px)] overflow-y-auto pr-1'>
          {loading ? (
            <div className='space-y-3'>
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className='rounded-lg border p-3 sm:p-4'>
                  <Skeleton className='h-4 w-2/3' />
                  <Skeleton className='mt-2 h-3 w-1/3' />
                </div>
              ))}
            </div>
          ) : records.length === 0 ? (
            <div className='text-muted-foreground flex min-h-40 flex-col items-center justify-center py-10 text-center'>
              <p className='text-sm font-medium'>
                {t('No redemption records found')}
              </p>
            </div>
          ) : (
            <div className='space-y-3'>
              {records.map((record) => (
                <div key={record.id} className='rounded-lg border p-3 sm:p-4'>
                  <div className='text-foreground text-sm'>{record.content}</div>
                  {record.created_at !== undefined && (
                    <div className='text-muted-foreground mt-1 text-xs'>
                      {new Date(record.created_at * 1000).toLocaleString()}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {!loading && records.length > 0 && (
          <div className='flex flex-col items-center gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
            <div className='text-muted-foreground text-xs sm:text-sm'>
              {t('Showing')} {(page - 1) * pageSize + 1}-
              {Math.min(page * pageSize, total)} {t('of')} {total}
            </div>
            <div className='flex items-center gap-2'>
              <button
                type='button'
                onClick={() => handlePageChange(page - 1)}
                disabled={page <= 1}
                className='h-8 w-8 rounded border disabled:opacity-50'
              >
                <ChevronLeft className='mx-auto h-4 w-4' />
              </button>
              <div className='text-muted-foreground flex items-center gap-1 text-sm'>
                <span className='font-medium'>{page}</span>
                <span>/</span>
                <span>{totalPages}</span>
              </div>
              <button
                type='button'
                onClick={() => handlePageChange(page + 1)}
                disabled={page >= totalPages}
                className='h-8 w-8 rounded border disabled:opacity-50'
              >
                <ChevronRight className='mx-auto h-4 w-4' />
              </button>
              <select
                value={pageSize.toString()}
                onChange={(e) => handlePageSizeChange(parseInt(e.target.value))}
                className='h-8 rounded border bg-transparent px-2 text-sm'
              >
                {[10, 20, 50, 100].map((n) => (
                  <option key={n} value={n.toString()}>
                    {n} / {t('page')}
                  </option>
                ))}
              </select>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  )
}
