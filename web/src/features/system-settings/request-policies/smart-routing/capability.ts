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
import type { BuiltinThinkingCapabilities } from './api'
import { SMART_TIER_IDS } from './lib'
import type {
  SmartEffort,
  SmartRoute,
  SmartRouteTier,
  ThinkingCapabilityOverride,
} from './types'

export const SMART_EFFORTS: SmartEffort[] = [
  'none',
  'minimal',
  'low',
  'medium',
  'high',
  'xhigh',
  'max',
]

const UNCONFIRMED_EFFORTS: SmartEffort[] = ['low', 'medium', 'high']

export type EffectiveCapability = {
  source: 'builtin' | 'override' | 'unconfirmed'
  noThinking: boolean
  efforts: SmartEffort[]
}

export function effectiveCapability(
  model: string,
  builtin: BuiltinThinkingCapabilities,
  overrides: Record<string, ThinkingCapabilityOverride>
): EffectiveCapability {
  const override = overrides[model]
  if (override) {
    return {
      source: 'override',
      noThinking: override.no_thinking,
      efforts: override.efforts,
    }
  }
  const known = builtin[model]
  if (!known?.known) {
    return {
      source: 'unconfirmed',
      noThinking: false,
      efforts: UNCONFIRMED_EFFORTS,
    }
  }
  return {
    source: 'builtin',
    noThinking: known.no_thinking,
    efforts: known.efforts,
  }
}

export function clampEffort(
  effort: SmartEffort,
  capability: EffectiveCapability
): { value: SmartEffort | ''; changed: boolean } {
  if (capability.noThinking || capability.efforts.length === 0) {
    return { value: '', changed: true }
  }
  if (capability.efforts.includes(effort)) {
    return { value: effort, changed: false }
  }
  const rank = SMART_EFFORTS.indexOf(effort)
  let best = capability.efforts[0]
  for (const candidate of capability.efforts) {
    const distance = Math.abs(SMART_EFFORTS.indexOf(candidate) - rank)
    const bestDistance = Math.abs(SMART_EFFORTS.indexOf(best) - rank)
    const higherOnTie =
      distance === bestDistance &&
      SMART_EFFORTS.indexOf(candidate) > SMART_EFFORTS.indexOf(best)
    if (distance < bestDistance || higherOnTie) best = candidate
  }
  return { value: best, changed: true }
}

export function effortOverflow(
  tier: SmartRouteTier,
  capability: EffectiveCapability
): { overflow: boolean; actual: SmartEffort | '' } {
  let stored: (SmartEffort | '')[] = []
  if (tier.effort_mode === 'fixed') stored = [tier.effort]
  if (tier.effort_mode === 'follow') stored = [tier.effort_min, tier.effort_max]
  const highest = stored.at(-1) ?? ''
  if (!highest || capability.noThinking) {
    return { overflow: false, actual: highest }
  }
  const outOfRange = stored.filter(
    (value): value is SmartEffort =>
      value !== '' && !capability.efforts.includes(value)
  )
  if (outOfRange.length === 0) return { overflow: false, actual: highest }
  const reported = outOfRange.at(-1) as SmartEffort
  return { overflow: true, actual: clampEffort(reported, capability).value }
}

export function capabilityModelsOfRoutes(routes: SmartRoute[]) {
  const models = routes.flatMap((route) => [
    ...SMART_TIER_IDS.map((id) => route.tiers[id].model),
    route.scorer.model,
  ])
  return [...new Set(models.filter(Boolean))]
}
