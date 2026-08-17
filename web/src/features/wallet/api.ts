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
import { api } from '@/lib/api'

import type {
  RedemptionRequest,
  AffiliateTransferRequest,
  ApiResponse,
  RedemptionResponse,
  AffiliateCodeResponse,
  AffiliateTransferResponse,
} from './types'

export function isApiSuccess(response: ApiResponse): boolean {
  return response.success === true || response.message === 'success'
}

/**
 * Redeem a topup code
 */
export async function redeemTopupCode(
  request: RedemptionRequest,
): Promise<RedemptionResponse> {
  const res = await api.post('/api/user/topup', request)
  return res.data
}

/**
 * Get affiliate code
 */
export async function getAffiliateCode(): Promise<AffiliateCodeResponse> {
  const res = await api.get('/api/user/aff')
  return res.data
}

/**
 * Transfer affiliate quota to balance
 */
export async function transferAffiliateQuota(
  request: AffiliateTransferRequest,
): Promise<AffiliateTransferResponse> {
  const res = await api.post('/api/user/aff_transfer', request)
  return res.data
}

/**
 * Get user logs filtered by type (e.g. redemption records via LogTypeTopup).
 */
export async function getUserLogs(
  page: number,
  pageSize: number,
  logType?: number,
): Promise<ApiResponse<unknown>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  if (logType !== undefined) {
    params.append('type', logType.toString())
  }
  const res = await api.get(`/api/user/log?${params.toString()}`)
  return res.data
}
