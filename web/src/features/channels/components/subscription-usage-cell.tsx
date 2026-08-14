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
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import { Progress } from '@/components/ui/progress'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  getCodexUsage,
  getSubscriptionUsage,
  type SubscriptionUsageEntry,
} from '../api'
import {
  estimateNextSubscriptionRefresh,
  formatRelativeTime,
  isSubscriptionUsageExpired,
} from '../lib'
import {
  CodexUsageDialog,
  type CodexUsageDialogData,
} from './dialogs/codex-usage-dialog'

export function useSubscriptionUsage() {
  return useQuery({
    queryKey: ['subscription-usage'],
    queryFn: async () => {
      try {
        const res = await getSubscriptionUsage()
        if (!res.success) {
          return {}
        }
        return res.data ?? {}
      } catch {
        // 用量是辅助信息：请求失败必须静默降级为空数据，
        // 不能冒泡到全局 queryCache.onError 把用户跳转到错误页
        return {}
      }
    },
    retry: false,
    staleTime: 30_000,
  })
}

type SubscriptionUsageCellProps = {
  channel: { id: number; type: number }
  usage: SubscriptionUsageEntry | undefined
  channelName?: string
  channelDisplayName?: string
  channelDisplayId?: string
}

export function SubscriptionUsageCell({
  channel,
  usage,
  channelName,
  channelDisplayName,
  channelDisplayId,
}: SubscriptionUsageCellProps) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [codexResponse, setCodexResponse] =
    useState<CodexUsageDialogData | null>(null)

  if (!usage) {
    return (
      <StatusBadge
        label={t('No data yet')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  const isClickable = channel.type === 57
  const expired = isSubscriptionUsageExpired(usage.refreshed_at, Date.now())
  const nextRefresh = estimateNextSubscriptionRefresh(
    usage.refreshed_at,
    channel.type
  )

  const handleClick = async () => {
    if (!isClickable || isLoading) {
      return
    }
    setIsLoading(true)
    try {
      const res = await getCodexUsage(channel.id)
      if (!res.success) {
        throw new Error(res.message || t('Failed to fetch usage'))
      }
      setCodexResponse(res)
      setDialogOpen(true)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to fetch usage')
      )
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <TooltipProvider>
      <div className='-ml-1.5 flex items-center gap-1.5'>
        <Tooltip>
          <TooltipTrigger
            render={
              <div
                className={cn(
                  'flex items-center gap-1.5',
                  isClickable && 'cursor-pointer'
                )}
                onClick={isClickable ? handleClick : undefined}
              />
            }
          >
            <span className='text-xs font-medium tabular-nums'>
              {Math.round(usage.bottleneck_percent * 10) / 10}%
            </span>
            <Progress value={usage.bottleneck_percent} className='h-1.5 w-16' />
          </TooltipTrigger>
          <TooltipContent>
            <div className='space-y-1 text-xs'>
              <div>{t('Saturation threshold: 95')}</div>
              <div>
                {t('Last refreshed')}:{' '}
                {formatTimestampToDate(usage.refreshed_at, 'milliseconds')} (
                {formatRelativeTime(usage.refreshed_at / 1000)})
              </div>
              <div>
                {t('Estimated next refresh (not guaranteed)')}:{' '}
                {formatTimestampToDate(nextRefresh, 'milliseconds')}
              </div>
              {isClickable && <div>{t('Click to view Codex usage')}</div>}
            </div>
          </TooltipContent>
        </Tooltip>
        {expired && (
          <StatusBadge label={t('Stale')} variant='warning' copyable={false} />
        )}
      </div>
      {isClickable && (
        <CodexUsageDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          channelName={channelName}
          channelId={channel.id}
          channelDisplayName={channelDisplayName}
          channelDisplayId={channelDisplayId}
          response={codexResponse}
          onRefresh={handleClick}
          isRefreshing={isLoading}
        />
      )}
    </TooltipProvider>
  )
}

type SubscriptionSaturationBadgeProps = {
  usage: SubscriptionUsageEntry | undefined
}

export function SubscriptionSaturationBadge({
  usage,
}: SubscriptionSaturationBadgeProps) {
  const { t } = useTranslation()

  if (!usage?.saturated) {
    return null
  }

  return (
    <StatusBadge label={t('Saturated')} variant='danger' copyable={false} />
  )
}
