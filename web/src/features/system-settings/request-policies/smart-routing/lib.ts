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
import type { TFunction } from 'i18next'

import type { SmartRoute, SmartTierId } from './types'

export const SMART_ROUTES_OPTION_KEY = 'smart_routing_setting.routes'

export const SMART_TIER_IDS: SmartTierId[] = ['simple', 'normal', 'hard']

const ENDPOINT_NAMES: [string, string][] = [
  ['openai-response', 'Responses'],
  ['openai', 'Chat Completions'],
]

type SmartRoutesParseError = 'invalid' | 'not-array' | 'incomplete'

export function parseSmartRoutes(text: string): {
  routes: SmartRoute[]
  error: SmartRoutesParseError | null
} {
  if (!text.trim()) return { routes: [], error: null }
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    return { routes: [], error: 'invalid' }
  }
  if (!Array.isArray(parsed)) return { routes: [], error: 'not-array' }
  if (!parsed.every(isCompleteRoute)) return { routes: [], error: 'incomplete' }
  return { routes: parsed as SmartRoute[], error: null }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function hasGroups(value: unknown) {
  return isObject(value) && Array.isArray(value.groups)
}

function isCompleteRoute(route: unknown) {
  if (!isObject(route) || !isObject(route.tiers)) return false
  const tiers = route.tiers
  return (
    SMART_TIER_IDS.every((id) => hasGroups(tiers[id])) &&
    hasGroups(route.scorer) &&
    hasGroups(route.advisor)
  )
}

export function smartRoutesErrorMessage(
  t: TFunction,
  error: SmartRoutesParseError
) {
  if (error === 'not-array') return t('Smart routing JSON must be an array')
  if (error === 'incomplete') {
    return t(
      'Each virtual model in the JSON must include tiers, scorer and advisor'
    )
  }
  return t('Invalid smart routing JSON')
}

export function serializeSmartRoutes(routes: SmartRoute[], indent?: number) {
  return JSON.stringify(routes, null, indent)
}

export function routeEndpoints(
  route: SmartRoute,
  models: { model_name: string; supported_endpoint_types?: string[] }[]
): string[] {
  const endpointsByModel = new Map(
    models.map((model) => [model.model_name, model.supported_endpoint_types])
  )
  const used = [
    ...SMART_TIER_IDS.map((id) => route.tiers[id].model),
    route.advisor.model,
  ]
  let common: Set<string> | null = null
  for (const name of used) {
    const endpoints = endpointsByModel.get(name)
    if (!endpoints) continue
    common = new Set(
      common ? endpoints.filter((e) => common?.has(e)) : endpoints
    )
  }
  const shared = common
  return ENDPOINT_NAMES.filter(([type]) => shared?.has(type)).map(
    ([, label]) => label
  )
}
