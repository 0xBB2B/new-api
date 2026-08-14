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
/* eslint-disable react-refresh/only-export-components */
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
import { toIntlLocale } from '@/i18n/languages'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  getClaudeUsage,
  getCodexUsage,
  getSubscriptionUsage,
  type ClaudeUsageResponse,
  type SubscriptionUsageEntry,
} from '../api'
import { SUBSCRIPTION_CHANNEL_TYPES } from '../constants'
import {
  estimateNextSubscriptionRefresh,
  formatRelativeTime,
  getUsagePercentLevel,
  isSubscriptionUsageExpired,
} from '../lib'
import { ClaudeUsageDialog } from './dialogs/claude-usage-dialog'
import {
  CodexUsageDialog,
  type CodexUsageDialogData,
} from './dialogs/codex-usage-dialog'

const percentTextClassName: Record<
  ReturnType<typeof getUsagePercentLevel>,
  string
> = {
  danger: 'text-rose-500',
  warning: 'text-amber-500',
  default: '',
}

const progressIndicatorClassName: Record<
  ReturnType<typeof getUsagePercentLevel>,
  string
> = {
  danger: '[&_[data-slot=progress-indicator]]:bg-rose-500',
  warning: '[&_[data-slot=progress-indicator]]:bg-amber-500',
  default: '',
}

type SubscriptionUsageTooltipContentProps = {
  usage: SubscriptionUsageEntry
  channelType: number
  locale: string | undefined
  nextRefresh: number
  isClickable: boolean
}

function SubscriptionUsageTooltipContent({
  usage,
  channelType,
  locale,
  nextRefresh,
  isClickable,
}: SubscriptionUsageTooltipContentProps) {
  const { t } = useTranslation()

  return (
    <div className='space-y-1 text-xs'>
      <div>
        {t('Bottleneck usage')}:{' '}
        {Math.round(usage.bottleneck_percent * 10) / 10}%
      </div>
      <div>{t('Saturation threshold: 95')}</div>
      <div>
        {t('Last refreshed')}:{' '}
        {formatTimestampToDate(usage.refreshed_at, 'milliseconds')} (
        {formatRelativeTime(usage.refreshed_at / 1000, locale)})
      </div>
      <div>
        {t('Estimated next refresh (not guaranteed)')}:{' '}
        {formatTimestampToDate(nextRefresh, 'milliseconds')}
      </div>
      {isClickable && channelType === 57 && (
        <div>{t('Click to view Codex usage')}</div>
      )}
      {isClickable && channelType === 61 && (
        <div>{t('Click to view Claude usage')}</div>
      )}
    </div>
  )
}

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
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [isLoading, setIsLoading] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [codexResponse, setCodexResponse] =
    useState<CodexUsageDialogData | null>(null)
  const [claudeResponse, setClaudeResponse] =
    useState<ClaudeUsageResponse | null>(null)

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

  const isClickable = SUBSCRIPTION_CHANNEL_TYPES.has(channel.type)
  const expired = isSubscriptionUsageExpired(usage.refreshed_at, Date.now())
  const nextRefresh = estimateNextSubscriptionRefresh(
    usage.refreshed_at,
    channel.type
  )
  const percentLevel = getUsagePercentLevel(usage.bottleneck_percent)

  const handleDialogOpenChange = (open: boolean) => {
    setDialogOpen(open)
    if (!open) {
      setCodexResponse(null)
      setClaudeResponse(null)
    }
  }

  const handleClick = async () => {
    if (!isClickable || isLoading) {
      return
    }
    setIsLoading(true)
    try {
      if (channel.type === 57) {
        const res = await getCodexUsage(channel.id)
        if (!res.success) {
          throw new Error(res.message || t('Failed to fetch usage'))
        }
        setCodexResponse(res)
      } else {
        const res = await getClaudeUsage(channel.id)
        if (!res.success) {
          if (!dialogOpen) {
            throw new Error(res.message || t('Failed to fetch usage'))
          }
          // 弹窗已打开的刷新失败交给弹窗错误横幅展示，不降级为 toast
          setClaudeResponse(res)
          return
        }
        setClaudeResponse(res)
      }
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
      <div className='flex items-center gap-1.5'>
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
            <Progress
              value={usage.bottleneck_percent}
              className={cn(
                'h-1.5 w-16',
                progressIndicatorClassName[percentLevel]
              )}
            />
            <span
              className={cn(
                'text-xs font-medium tabular-nums',
                percentTextClassName[percentLevel]
              )}
            >
              {Math.round(usage.bottleneck_percent * 10) / 10}%
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <SubscriptionUsageTooltipContent
              usage={usage}
              channelType={channel.type}
              locale={locale}
              nextRefresh={nextRefresh}
              isClickable={isClickable}
            />
          </TooltipContent>
        </Tooltip>
        {expired && (
          <StatusBadge label={t('Stale')} variant='warning' copyable={false} />
        )}
      </div>
      {isClickable && channel.type === 57 && (
        <CodexUsageDialog
          open={dialogOpen}
          onOpenChange={handleDialogOpenChange}
          channelName={channelName}
          channelId={channel.id}
          channelDisplayName={channelDisplayName}
          channelDisplayId={channelDisplayId}
          response={codexResponse}
          onRefresh={handleClick}
          isRefreshing={isLoading}
        />
      )}
      {isClickable && channel.type === 61 && (
        <ClaudeUsageDialog
          open={dialogOpen}
          onOpenChange={handleDialogOpenChange}
          channelName={channelName}
          channelId={channel.id}
          channelDisplayName={channelDisplayName}
          channelDisplayId={channelDisplayId}
          response={claudeResponse}
          onRefresh={handleClick}
          isRefreshing={isLoading}
        />
      )}
    </TooltipProvider>
  )
}

type SubscriptionSaturationBadgeProps = {
  usage: SubscriptionUsageEntry | undefined
  channelType: number
}

export function SubscriptionSaturationBadge({
  usage,
  channelType,
}: SubscriptionSaturationBadgeProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  if (!usage?.saturated) {
    return null
  }

  const nextRefresh = estimateNextSubscriptionRefresh(
    usage.refreshed_at,
    channelType
  )

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger render={<div className='inline-flex' />}>
          <StatusBadge
            label={t('Saturated')}
            variant='danger'
            copyable={false}
          />
        </TooltipTrigger>
        <TooltipContent>
          <SubscriptionUsageTooltipContent
            usage={usage}
            channelType={channelType}
            locale={locale}
            nextRefresh={nextRefresh}
            isClickable={false}
          />
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
