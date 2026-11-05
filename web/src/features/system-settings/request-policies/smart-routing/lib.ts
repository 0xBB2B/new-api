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

import type { EffectiveCapability } from './capability'
import type {
  SmartEndpoint,
  SmartRoute,
  SmartRouteTier,
  SmartTierId,
  ThinkingCapabilityOverride,
} from './types'

export const SMART_ROUTES_OPTION_KEY = 'smart_routing_setting.routes'

export const SMART_THINKING_CAPABILITIES_OPTION_KEY =
  'smart_routing_setting.thinking_capabilities'

export const SMART_TIER_IDS: SmartTierId[] = ['simple', 'normal', 'hard']

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

export function parseThinkingCapabilities(
  text: string
): Record<string, ThinkingCapabilityOverride> {
  if (!text.trim()) return {}
  try {
    const parsed: unknown = JSON.parse(text)
    return isObject(parsed)
      ? (parsed as Record<string, ThinkingCapabilityOverride>)
      : {}
  } catch {
    return {}
  }
}

export function serializeThinkingCapabilities(
  map: Record<string, ThinkingCapabilityOverride>,
  indent?: number
) {
  return JSON.stringify(map, null, indent)
}

const ENDPOINT_SHORT_NAMES: Record<SmartEndpoint, string> = {
  'openai-response': 'Responses',
  openai: 'Chat',
  anthropic: 'Anthropic',
}

export function endpointShortNames(endpoints: SmartEndpoint[]) {
  return endpoints.map((endpoint) => ENDPOINT_SHORT_NAMES[endpoint]).join(' / ')
}

export function tierEffortSummary(
  tier: SmartRouteTier,
  capability: EffectiveCapability,
  t: TFunction
) {
  if (capability.noThinking) return t('No thinking')
  if (!tier.effort_mode) return t('Not overridden')
  if (tier.effort_mode === 'fixed') {
    return t('Fixed {{effort}}', { effort: tier.effort })
  }
  return t('Follow scoring {{min}}～{{max}}', {
    min: tier.effort_min,
    max: tier.effort_max,
  })
}

export function slotSummaries(route: SmartRoute, t: TFunction) {
  const tierCount = SMART_TIER_IDS.filter((id) => route.tiers[id].model).length
  return {
    scorer: route.scorer.model || t('Not set'),
    tiers: t('{{count}}/3 set', { count: tierCount }),
    advisor: route.advisor.model || t('Not set'),
  }
}
