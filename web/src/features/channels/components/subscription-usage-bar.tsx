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

function UsageRow({
  label,
  percent,
  variant,
  ariaLabel,
}: {
  label: string
  percent: number
  variant: UsageVariant
  ariaLabel: string
}) {
  const classes = usageVariantClassName[variant]
  return (
    <>
      <span className='text-muted-foreground w-4 text-[10px]'>{label}</span>
      <span className={cn('w-9 text-right text-xs tabular-nums', classes.text)}>
        {Math.round(percent)}%
      </span>
      <Progress
        value={percent}
        aria-label={ariaLabel}
        className={cn('w-14 gap-0', classes.indicator)}
      />
    </>
  )
}

export function SubscriptionUsageBar({ channel }: { channel: Channel }) {
  const { t } = useTranslation()
  const snapshot = parseSubscriptionUsageSnapshot(channel.other_info)
  const { fiveHourWindow, weeklyWindow } = resolveRateLimitWindows(
    snapshot ? { plan_type: snapshot.plan_type, rate_limit: snapshot } : null
  )

  if (!fiveHourWindow && !weeklyWindow) {
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

  const updatedAt = Number(snapshot?.updated_at)
  const fiveHour = windowLabel(fiveHourWindow)
  const weekly = windowLabel(weeklyWindow)

  const content = (
    <div className='grid cursor-help grid-cols-[auto_auto_auto] items-center gap-x-1.5 gap-y-0.5'>
      {fiveHourWindow && (
        <UsageRow
          label='5h'
          {...fiveHour}
          ariaLabel={`${t('5-Hour Window')}: ${Math.round(fiveHour.percent)}%`}
        />
      )}
      {weeklyWindow && (
        <UsageRow
          label='7d'
          {...weekly}
          ariaLabel={`${t('Weekly Window')}: ${Math.round(weekly.percent)}%`}
        />
      )}
    </div>
  )

  return (
    <Tooltip>
      <TooltipTrigger render={content} />
      <TooltipContent>
        {Number.isFinite(updatedAt) && updatedAt > 0 && (
          <p>
            {t('Updated at:')} {formatTimestampToDate(updatedAt)}
          </p>
        )}
      </TooltipContent>
    </Tooltip>
  )
}
