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
import { api } from '@/lib/api'

export interface ProxySummary {
  id: number
  name: string
  protocol: string
  host: string
  port: number
  status: 'active' | 'inactive' | string
  latency_ms?: number | null
  ip_address?: string
  country?: string
  country_code?: string
  region?: string
  city?: string
  quality_status?: string
  quality_score?: number | null
  quality_grade?: string
  quality_summary?: string
  last_checked_at?: number | null
  last_http_status?: number | null
  last_error?: string
  credential_configured: boolean
  credential_decrypt_failed: boolean
  bound_channel_count: number
  quality_items?: ProxyQualityItem[]
}

export interface ProxyQualityItem {
  target: string
  url?: string
  status: 'pass' | 'warn' | 'challenge' | 'fail' | string
  http_status?: number
  latency_ms?: number
  message?: string
  cf_ray?: string
}

export interface ProxyListResponse {
  success: boolean
  message?: string
  data?: {
    items: ProxySummary[]
    total: number
    page: number
    page_size: number
  }
}

export interface ProxyWriteRequest {
  name: string
  protocol: string
  host: string
  port: number
  username?: string
  password?: string
  status?: string
  clear_credentials?: boolean
}

export interface QuickAddItem {
  line: number
  created: boolean
  skipped: boolean
  proxy?: ProxySummary
  error?: string
}

export async function listProxies(
  params: {
    p?: number
    page_size?: number
    status?: string
    search?: string
  } = {}
) {
  const response = await api.get<ProxyListResponse>('/api/proxy/', { params })
  return response.data
}

export async function listActiveProxies() {
  const response = await api.get<{ success: boolean; data?: ProxySummary[] }>(
    '/api/proxy/all'
  )
  return response.data
}

export async function createProxy(payload: ProxyWriteRequest) {
  const response = await api.post<{
    success: boolean
    message?: string
    data?: ProxySummary
  }>('/api/proxy/', payload)
  return response.data
}

export async function updateProxy(
  id: number,
  payload: Partial<ProxyWriteRequest>
) {
  const response = await api.put<{
    success: boolean
    message?: string
    data?: ProxySummary
  }>(`/api/proxy/${id}`, payload)
  return response.data
}

export async function deleteProxy(id: number) {
  const response = await api.delete<{
    success: boolean
    message?: string
    data?: { bound_channel_count: number }
  }>(`/api/proxy/${id}`)
  return response.data
}

export async function quickAddProxies(urls: string[]) {
  const response = await api.post<{
    success: boolean
    message?: string
    data?: { items: QuickAddItem[] }
  }>('/api/proxy/quick-add', { urls })
  return response.data
}

export async function testProxy(id: number) {
  const response = await api.post<{
    success: boolean
    message?: string
    data?: ProxySummary
  }>(`/api/proxy/${id}/test`)
  return response.data
}

export async function qualityCheckProxy(id: number) {
  const response = await api.post<{
    success: boolean
    message?: string
    data?: ProxySummary
  }>(`/api/proxy/${id}/quality-check`)
  return response.data
}
