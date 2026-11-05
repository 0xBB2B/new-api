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
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import {
  isSmartRouteScoringFailed,
  smartRouteTierText,
} from '../lib/smart-route'
import type { SmartRouting } from '../types'

export function SmartRouteCaption(props: { routing?: SmartRouting }) {
  const { t } = useTranslation()
  const routing = props.routing
  if (!routing) return null

  if (routing.role === 'scorer') {
    return (
      <span className='text-muted-foreground text-xs'>
        {`${t('Routing score')} · ${routing.virtual_model}`}
      </span>
    )
  }

  const failed = isSmartRouteScoringFailed(routing)
  const parts = [routing.virtual_model, smartRouteTierText(t, routing.tier)]
  if (failed) parts.push(t('Scoring failed'))
  return (
    <span
      className={cn(
        'text-xs',
        failed && 'text-destructive',
        !failed && routing.tier === 'advisor' && 'text-warning',
        !failed && routing.tier !== 'advisor' && 'text-muted-foreground'
      )}
    >
      {parts.join(' · ')}
    </span>
  )
}
