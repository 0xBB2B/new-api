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

import type { SmartRouting } from '../types'

const SCORING_FAILED_STATUSES = new Set(['timeout', 'error', 'invalid'])

export function isSmartRouteScoringFailed(routing: SmartRouting): boolean {
  return SCORING_FAILED_STATUSES.has(routing.scorer?.status ?? '')
}

export function smartRouteTierText(
  t: TFunction,
  tier: string | undefined
): string {
  const names: Record<string, string> = {
    simple: t('Simple tier'),
    normal: t('Normal tier'),
    hard: t('Hard tier'),
    advisor: t('Advisor'),
  }
  return names[tier ?? ''] ?? tier ?? '-'
}

const EFFORT_ORDER = [
  'none',
  'minimal',
  'low',
  'medium',
  'high',
  'xhigh',
  'max',
]

export function smartRouteEffortText(
  t: TFunction,
  effort: SmartRouting['effort']
): string {
  if (!effort) return ''
  if (effort.source === 'no_thinking') {
    return t('Not sent (model does not support thinking)')
  }
  if (effort.source === 'client_unset') {
    return t('Not sent (client did not specify)')
  }
  let source =
    {
      follow: t('Follow scoring'),
      fixed: t('Fixed', { context: 'routing' }),
      client: effort.from
        ? t('Not overridden')
        : t('Not overridden, sent by the client'),
    }[effort.source] ?? effort.source
  if (effort.from) {
    const lowered =
      EFFORT_ORDER.indexOf(effort.value) < EFFORT_ORDER.indexOf(effort.from)
    source = lowered
      ? t('{{source}}, lowered from {{from}}', { source, from: effort.from })
      : t('{{source}}, raised from {{from}}', { source, from: effort.from })
  }
  return t('{{effort}} ({{source}})', { effort: effort.value, source })
}
