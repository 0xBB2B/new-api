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
import { SMART_TIER_IDS } from './lib'
import type { SmartTierId } from './types'

export type SmartEditorTab = 'scoring' | 'tiers' | 'advisor'

export type ErrorLocation = { tab: SmartEditorTab; tier?: SmartTierId }

export function errorFields(errors: object, prefix = ''): string[] {
  return Object.entries(errors).flatMap(([key, value]) => {
    if (typeof value !== 'object' || value === null) return []
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof (value as { type?: unknown }).type === 'string') return [path]
    return errorFields(value, path)
  })
}

export function tabOfField(field: string): SmartEditorTab | null {
  if (field.startsWith('scorer.')) return 'scoring'
  if (field.startsWith('tiers.') || field === 'default_tier') return 'tiers'
  if (field.startsWith('advisor.')) return 'advisor'
  return null
}

export function firstErrorLocation(fields: string[]): ErrorLocation | null {
  if (fields.some((field) => tabOfField(field) === 'scoring')) {
    return { tab: 'scoring' }
  }
  const tier = SMART_TIER_IDS.find((id) =>
    fields.some((field) => field.startsWith(`tiers.${id}.`))
  )
  if (tier) return { tab: 'tiers', tier }
  if (fields.includes('default_tier')) return { tab: 'tiers' }
  if (fields.some((field) => tabOfField(field) === 'advisor')) {
    return { tab: 'advisor' }
  }
  return null
}
