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

import type { SmartEffort, SmartRoute } from './types'

type SmartRouteValidationError = {
  field: string
  message: { key: string; params?: Record<string, unknown> }
}

type ValidateResponse = {
  success: boolean
  message?: string
  data: { errors: SmartRouteValidationError[] }
}

export async function validateSmartRoute(body: { route: SmartRoute }) {
  const response = await api.post<ValidateResponse>(
    '/api/option/smart_routing/validate',
    body
  )
  return requireServerSuccess(response.data).data
}

export type BuiltinThinkingCapabilities = Record<
  string,
  { known: boolean; no_thinking: boolean; efforts: SmartEffort[] }
>

export async function getBuiltinThinkingCapabilities(models: string[]) {
  const response = await api.get<{
    success: boolean
    message?: string
    data: BuiltinThinkingCapabilities
  }>('/api/option/smart_routing/thinking_capabilities', {
    params: { models: models.join(',') },
  })
  return requireServerSuccess(response.data).data
}
