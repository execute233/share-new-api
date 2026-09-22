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
import { requireServerSuccess } from '@/lib/server-error-message'

export interface DashboardBucket {
  bucket: number
  user_id: number
  username: string
  model_name: string
  requests: number
  quota: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  incomplete_requests: number
  duration_seconds: number
  timed_requests: number
  updated_at: number
}

export interface DashboardSnapshot {
  started_at: number
  generated_at: number
  total: DashboardBucket
  today: DashboardBucket
  keys: number
  active_keys: number
  channels: number
  active_channels: number
  users: number
  new_users: number
  active_users: number
  trend: DashboardBucket[] | null
  models: DashboardBucket[] | null
  ranking: DashboardBucket[] | null
  users_trend: DashboardBucket[] | null
}

export async function getAdminDashboard(
  start: number,
  end: number,
  granularity: string
) {
  const response = await api.get<{ success: boolean; data: DashboardSnapshot }>(
    '/api/admin-dashboard',
    {
      params: { start_timestamp: start, end_timestamp: end, granularity },
    }
  )
  return requireServerSuccess(response.data).data
}

export function totalTokens(row: DashboardBucket) {
  return (
    row.input_tokens +
    row.output_tokens +
    row.cache_creation_tokens +
    row.cache_read_tokens
  )
}
