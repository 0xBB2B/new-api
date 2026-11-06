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
export type SmartTierId = 'simple' | 'normal' | 'hard'
export type SmartEndpoint = 'openai-response' | 'openai' | 'anthropic'
export type SmartEffort =
  | 'none'
  | 'minimal'
  | 'low'
  | 'medium'
  | 'high'
  | 'xhigh'
  | 'max'
export type SmartEffortMode = '' | 'fixed' | 'follow'
export type SmartRouteTier = {
  model: string
  groups: string[]
  effort_mode: SmartEffortMode
  effort: SmartEffort | ''
  effort_min: SmartEffort | ''
  effort_max: SmartEffort | ''
}
export type ThinkingCapabilityOverride = {
  no_thinking: boolean
  efforts: SmartEffort[]
}
export type SmartRoute = {
  name: string
  endpoints: SmartEndpoint[]
  tiers: Record<SmartTierId, SmartRouteTier>
  default_tier: SmartTierId
  scorer: { model: string; groups: string[]; timeout_ms: number }
  advisor: { model: string; groups: string[]; after_dissatisfied: number }
  downgrade_max_history_tokens: number
  confidence_threshold: number
  simple_max_message_chars: number
}
