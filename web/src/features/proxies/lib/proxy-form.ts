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
import { z } from 'zod'

import type { ProxySummary } from '../api'

export const PROXY_PROTOCOLS = [
  'http',
  'https',
  'socks5',
  'socks5h',
  'ss',
] as const

export const proxyFormSchema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  protocol: z.enum(PROXY_PROTOCOLS),
  host: z.string().trim().min(1, 'Host is required'),
  port: z
    .number()
    .int('Port must be an integer')
    .min(1, 'Port must be between 1 and 65535')
    .max(65535, 'Port must be between 1 and 65535'),
  username: z.string(),
  password: z.string(),
  status: z.enum(['active', 'inactive']),
})

export type ProxyFormValues = z.infer<typeof proxyFormSchema>

export const PROXY_FORM_DEFAULT_VALUES: ProxyFormValues = {
  name: '',
  protocol: 'http',
  host: '',
  port: 8080,
  username: '',
  password: '',
  status: 'active',
}

export function transformProxyToFormValues(
  proxy: ProxySummary
): ProxyFormValues {
  return {
    name: proxy.name,
    protocol: proxy.protocol as ProxyFormValues['protocol'],
    host: proxy.host,
    port: proxy.port,
    username: '',
    password: '',
    status: proxy.status === 'inactive' ? 'inactive' : 'active',
  }
}