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
import i18next from 'i18next'
import { useState, useEffect, useCallback, useRef } from 'react'
import { toast } from 'sonner'

import { getUserLogs, isApiSuccess } from '../api'
import type { RedemptionRecord } from '../types'

const LogTypeTopup = 1

export function useBillingHistory() {
  const [records, setRecords] = useState<RedemptionRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const requestIdRef = useRef(0)
  const [loading, setLoading] = useState(false)

  const fetchHistory = useCallback(async () => {
    const requestId = ++requestIdRef.current
    setLoading(true)
    try {
      const response = await getUserLogs(page, pageSize, LogTypeTopup)
      if (requestId !== requestIdRef.current) return

      if (isApiSuccess(response) && response.data) {
        const data = response.data as {
          items?: RedemptionRecord[]
          total?: number
        }
        setRecords(data.items || [])
        setTotal(data.total || 0)
      } else {
        setRecords([])
        setTotal(0)
      }
    } catch (error) {
      if (requestId !== requestIdRef.current) return
      // eslint-disable-next-line no-console
      console.error('Failed to fetch redemption history:', error)
      toast.error(i18next.t('Failed to load history'))
      setRecords([])
      setTotal(0)
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false)
      }
    }
  }, [page, pageSize])

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage)
  }, [])

  const handlePageSizeChange = useCallback((newPageSize: number) => {
    setPageSize(newPageSize)
    setPage(1)
  }, [])

  useEffect(() => {
    fetchHistory()
  }, [fetchHistory])

  return {
    records,
    total,
    page,
    pageSize,
    loading,
    handlePageChange,
    handlePageSizeChange,
    refresh: fetchHistory,
  }
}
