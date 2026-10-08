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
  parseSubscriptionUsageSnapshot,
  resolveRateLimitWindows,
  windowLabel,
  type UsageVariant,
} from '../lib/subscription-usage'
import type { Channel } from '../types'

const usageVariantClassName: Record<
  UsageVariant,
  { text: string; indicator: string }
> = {
  info: {
    text: 'text-muted-foreground',
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

function UsageRow({
  label,
  percent,
  variant,
  ariaLabel,
  resetAt,
}: {
  label: string
  percent: number
  variant: UsageVariant
  ariaLabel: string
  resetAt: number
}) {
  const { t } = useTranslation()
  const classes = usageVariantClassName[variant]
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <div className='flex items-center gap-x-1.5'>
            <span className='text-muted-foreground w-4 text-[10px]'>
              {label}
            </span>
            <Progress
              value={percent}
              aria-label={ariaLabel}
              className={cn(
                'w-20 gap-0 [&_[data-slot=progress-track]]:h-1.5',
                classes.indicator
              )}
            />
            <span className={cn('w-9 text-xs tabular-nums', classes.text)}>
              {Math.round(percent)}%
            </span>
          </div>
        }
      />
      <TooltipContent>
        <p>
          {Number.isFinite(resetAt) && resetAt > 0
            ? t('Resets at {{time}}', { time: formatTimestampToDate(resetAt) })
            : t('Reset timer not started')}
        </p>
      </TooltipContent>
    </Tooltip>
  )
}

export function SubscriptionUsageBar({
  channel,
  onOpen,
  isLoading = false,
}: {
  channel: Channel
  onOpen: () => void
  isLoading?: boolean
}) {
  const { t } = useTranslation()
  const snapshot = parseSubscriptionUsageSnapshot(channel.other_info)
  const { fiveHourWindow, weeklyWindow } = resolveRateLimitWindows(
    snapshot ? { plan_type: snapshot.plan_type, rate_limit: snapshot } : null
  )

  const entryProps = {
    role: 'button',
    tabIndex: 0,
    'aria-haspopup': 'dialog' as const,
    'aria-label': t('Usage'),
    'aria-busy': isLoading,
    onClick: () => {
      if (!isLoading) onOpen()
    },
    onKeyDown: (e: React.KeyboardEvent) => {
      if (e.key !== 'Enter' && e.key !== ' ') return
      e.preventDefault()
      if (!isLoading) onOpen()
    },
  }
  const entryClassName = cn(
    'focus-visible:ring-ring/50 rounded-md outline-none focus-visible:ring-2',
    isLoading ? 'cursor-wait opacity-50' : 'hover:bg-muted/60 cursor-pointer'
  )

  if (!fiveHourWindow && !weeklyWindow) {
    return (
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              {...entryProps}
              className={cn(
                'text-muted-foreground px-1 text-xs',
                entryClassName
              )}
            >
              -
            </span>
          }
        />
        <TooltipContent>
          <p>{t('No usage data, click to query')}</p>
        </TooltipContent>
      </Tooltip>
    )
  }

  const fiveHour = windowLabel(fiveHourWindow)
  const weekly = windowLabel(weeklyWindow)

  return (
    <div
      {...entryProps}
      className={cn('flex flex-col gap-y-0.5 p-0.5', entryClassName)}
    >
      {fiveHourWindow && (
        <UsageRow
          label='5h'
          {...fiveHour}
          ariaLabel={`${t('5-Hour Window')}: ${Math.round(fiveHour.percent)}%`}
          resetAt={Number(fiveHourWindow.reset_at)}
        />
      )}
      {weeklyWindow && (
        <UsageRow
          label='7d'
          {...weekly}
          ariaLabel={`${t('Weekly Window')}: ${Math.round(weekly.percent)}%`}
          resetAt={Number(weeklyWindow.reset_at)}
        />
      )}
    </div>
  )
}
