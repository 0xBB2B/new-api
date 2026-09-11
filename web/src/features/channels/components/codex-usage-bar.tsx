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

import { Progress } from '@/components/ui/progress'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  parseCodexUsageSnapshot,
  resolveRateLimitWindows,
  windowLabel,
  type CodexRateLimitWindow,
  type CodexUsageVariant,
} from '../lib/codex-usage'
import type { Channel } from '../types'

const usageVariantClassName: Record<
  CodexUsageVariant,
  { text: string; indicator: string }
> = {
  info: {
    text: 'text-info',
    indicator: '[&_[data-slot=progress-indicator]]:bg-info',
  },
  warning: {
    text: 'text-warning',
    indicator: '[&_[data-slot=progress-indicator]]:bg-warning',
  },
  danger: {
    text: 'text-destructive',
    indicator: '[&_[data-slot=progress-indicator]]:bg-destructive',
  },
}

function windowSummary(
  title: string,
  window: CodexRateLimitWindow | null,
  resetAtLabel: string
): string | null {
  if (!window) {
    return null
  }
  const { percent } = windowLabel(window)
  const resetAt = Number(window.reset_at)
  const reset =
    Number.isFinite(resetAt) && resetAt > 0
      ? ` · ${resetAtLabel} ${formatTimestampToDate(resetAt)}`
      : ''
  return `${title}: ${Math.round(percent)}%${reset}`
}

export function CodexUsageBar({ channel }: { channel: Channel }) {
  const { t } = useTranslation()
  const snapshot = parseCodexUsageSnapshot(channel.other_info)
  const { fiveHourWindow, weeklyWindow } = resolveRateLimitWindows(
    snapshot ? { plan_type: snapshot.plan_type, rate_limit: snapshot } : null
  )
  const primary = weeklyWindow ?? fiveHourWindow

  if (!primary) {
    return (
      <Tooltip>
        <TooltipTrigger
          render={
            <span className='text-muted-foreground cursor-help text-xs'>-</span>
          }
        />
        <TooltipContent>
          <p>{t('No usage data')}</p>
        </TooltipContent>
      </Tooltip>
    )
  }

  const { percent, variant } = windowLabel(primary)
  const classes = usageVariantClassName[variant]
  const lines = [
    windowSummary(t('5-Hour Window'), fiveHourWindow, t('Reset at:')),
    windowSummary(t('Weekly Window'), weeklyWindow, t('Reset at:')),
  ].filter(Boolean) as string[]
  const updatedAt = Number(snapshot?.updated_at)

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <div className='flex cursor-help items-center gap-1.5'>
            <Progress
              value={percent}
              aria-label={`${t('Weekly Window')}: ${Math.round(percent)}%`}
              className={cn('w-14 gap-0', classes.indicator)}
            />
            <span className={cn('text-xs tabular-nums', classes.text)}>
              {Math.round(percent)}%
            </span>
          </div>
        }
      />
      <TooltipContent>
        {lines.map((line) => (
          <p key={line}>{line}</p>
        ))}
        {Number.isFinite(updatedAt) && updatedAt > 0 && (
          <p>
            {t('Updated at:')} {formatTimestampToDate(updatedAt)}
          </p>
        )}
      </TooltipContent>
    </Tooltip>
  )
}
