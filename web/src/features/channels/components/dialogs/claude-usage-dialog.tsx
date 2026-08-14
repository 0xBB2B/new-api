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
import { Check, ChevronDown, ChevronUp, Copy, RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Progress } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { toIntlLocale } from '@/i18n/languages'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { ClaudeUsageResponse } from '../../api'
import { formatRelativeTime, getUsagePercentLevel } from '../../lib'

type ClaudeUsageWindow = {
  key: string
  label: string
  percent: number
  resetsAt?: number
}

const WINDOW_LABELS: Record<string, string> = {
  five_hour: '5-Hour Window',
  seven_day: 'Weekly Window',
  seven_day_opus: 'Weekly Opus Window',
  seven_day_sonnet: 'Weekly Sonnet Window',
}

function labelForWindowKey(key: string): string {
  return WINDOW_LABELS[key] ?? key
}

function extractResetsAt(entry: Record<string, unknown>): number | undefined {
  return typeof entry.resets_at === 'number' ? entry.resets_at : undefined
}

export function resolveClaudeUsageWindows(
  data: unknown
): ClaudeUsageWindow[] {
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    return []
  }
  const obj = data as Record<string, unknown>

  if (Array.isArray(obj.limits)) {
    return (obj.limits as Array<Record<string, unknown>>)
      .filter((item) => item && typeof item === 'object')
      .map((item) => {
        const key = String(item.kind ?? '')
        return {
          key,
          label: labelForWindowKey(key),
          percent: Number(item.percent) || 0,
          resetsAt: extractResetsAt(item),
        }
      })
  }

  return Object.entries(obj)
    .filter(
      ([key, value]) =>
        (key === 'five_hour' || key.startsWith('seven_day')) &&
        value &&
        typeof value === 'object'
    )
    .map(([key, value]) => {
      const entry = value as Record<string, unknown>
      return {
        key,
        label: labelForWindowKey(key),
        percent: Number(entry.utilization) || 0,
        resetsAt: extractResetsAt(entry),
      }
    })
}

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

function InfoField(props: {
  label: string
  value: string
  className?: string
}) {
  return (
    <div
      className={cn(
        'bg-background ring-border/60 min-w-0 rounded-lg p-3 ring-1',
        props.className
      )}
    >
      <div className='text-muted-foreground text-[11px] font-medium'>
        {props.label}
      </div>
      <div className='mt-1 text-xs leading-5 break-all'>{props.value}</div>
    </div>
  )
}

type ClaudeUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelName?: string
  channelId?: number
  channelDisplayName?: string
  channelDisplayId?: string
  response: ClaudeUsageResponse | null
  onRefresh?: () => void | Promise<void>
  isRefreshing?: boolean
}

export function ClaudeUsageDialog({
  open,
  onOpenChange,
  channelName,
  channelId,
  channelDisplayName,
  channelDisplayId,
  response,
  onRefresh,
  isRefreshing,
}: ClaudeUsageDialogProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const [showRawJson, setShowRawJson] = useState(false)

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setShowRawJson(false)
    }
    onOpenChange(nextOpen)
  }

  const windows = useMemo(
    () => resolveClaudeUsageWindows(response?.data),
    [response?.data]
  )

  const channelLabelName = channelDisplayName ?? channelName ?? '-'
  const channelLabelId = channelDisplayId ?? (channelId ? String(channelId) : '')
  const channelLabel = channelLabelId
    ? `${channelLabelName} (#${channelLabelId})`
    : channelLabelName

  const dataFields =
    response?.data &&
    typeof response.data === 'object' &&
    !Array.isArray(response.data)
      ? (response.data as Record<string, unknown>)
      : {}
  const emailValue =
    (typeof dataFields.email === 'string' && dataFields.email.trim()) || '-'
  const userIdValue =
    (typeof dataFields.user_id === 'string' && dataFields.user_id.trim()) || '-'

  const errorMessage =
    response?.success === false
      ? response?.message?.trim() || t('Failed to fetch usage')
      : ''

  const rawJsonText = useMemo(
    () => (response ? JSON.stringify(response, null, 2) : ''),
    [response]
  )

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t('Claude Account & Usage')}
      contentClassName='sm:max-w-[900px]'
      titleClassName='flex items-center gap-2'
      contentHeight='auto'
      bodyClassName='flex flex-col gap-4'
      footer={
        <Button
          type='button'
          variant='outline'
          onClick={() => handleOpenChange(false)}
        >
          {t('Close')}
        </Button>
      }
    >
      <div className='flex flex-col gap-4'>
        {errorMessage && (
          <div className='rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400'>
            {errorMessage}
          </div>
        )}

        <Card size='sm' className='bg-muted/30 gap-0 py-0'>
          <CardHeader className='p-4 pb-2'>
            <CardTitle className='text-muted-foreground text-xs font-medium'>
              {t('Claude Account Status')}
            </CardTitle>
            {onRefresh ? (
              <CardAction>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={onRefresh}
                  disabled={Boolean(isRefreshing)}
                >
                  <RefreshCw data-icon='inline-start' />
                  {t('Refresh')}
                </Button>
              </CardAction>
            ) : null}
          </CardHeader>
          <CardContent className='p-4 pt-0'>
            <div className='flex flex-wrap items-center gap-2'>
              {response?.subscription_type ? (
                <StatusBadge
                  label={response.subscription_type}
                  variant='blue'
                  copyable={false}
                />
              ) : null}
              <StatusBadge
                label={`HTTP ${response?.upstream_status ?? '-'}`}
                variant='neutral'
                copyable={false}
              />
            </div>
            <div className='mt-4 grid grid-cols-1 gap-3 md:grid-cols-2'>
              <InfoField label={t('Email')} value={emailValue} />
              <InfoField label={t('Channel')} value={channelLabel} />
              <InfoField
                label={t('User ID')}
                value={userIdValue}
                className='md:col-span-2'
              />
            </div>
          </CardContent>
        </Card>

        <div className='flex flex-col gap-3'>
          <div className='text-sm font-semibold'>{t('Base Limits')}</div>
          <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
            {windows.map((window) => {
              const level = getUsagePercentLevel(window.percent)
              return (
                <Card key={window.key} size='sm' className='gap-0 py-0'>
                  <CardHeader className='p-3 pb-2'>
                    <div className='flex items-start justify-between gap-3'>
                      <CardTitle className='text-sm font-semibold'>
                        {t(window.label)}
                      </CardTitle>
                      <div className='shrink-0 text-right'>
                        <div
                          className={cn(
                            'text-xl leading-none font-semibold tabular-nums',
                            percentTextClassName[level]
                          )}
                        >
                          {window.percent}%
                        </div>
                        <div className='text-muted-foreground mt-1 text-[11px]'>
                          {t('Used')}
                        </div>
                      </div>
                    </div>
                    <div className='text-muted-foreground mt-1 text-xs'>
                      {t('Window:')} -
                    </div>
                  </CardHeader>
                  <CardContent className='p-3 pt-0'>
                    <Progress
                      value={window.percent}
                      className={progressIndicatorClassName[level]}
                    />
                    <div className='mt-3 grid grid-cols-1 gap-2 text-xs sm:grid-cols-2'>
                      <div className='min-w-0'>
                        <div className='text-muted-foreground text-[11px]'>
                          {t('Reset at:')}
                        </div>
                        <div className='break-all tabular-nums'>
                          {window.resetsAt != null
                            ? formatTimestampToDate(window.resetsAt)
                            : '-'}
                        </div>
                      </div>
                      <div className='min-w-0 sm:text-right'>
                        <div className='text-muted-foreground text-[11px]'>
                          {t('Resets in:')}
                        </div>
                        <div className='tabular-nums'>
                          {window.resetsAt != null
                            ? formatRelativeTime(window.resetsAt, locale)
                            : '-'}
                        </div>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              )
            })}
          </div>
        </div>

        <Collapsible
          open={showRawJson}
          onOpenChange={setShowRawJson}
          className='rounded-lg border'
        >
          <CollapsibleTrigger
            render={
              <button
                type='button'
                className='hover:bg-muted/40 flex w-full items-center justify-between gap-2 p-3 transition-colors'
                aria-expanded={showRawJson}
              />
            }
          >
            <div className='text-sm font-medium'>{t('Raw JSON')}</div>
            {showRawJson ? (
              <ChevronUp className='text-muted-foreground h-4 w-4' />
            ) : (
              <ChevronDown className='text-muted-foreground h-4 w-4' />
            )}
          </CollapsibleTrigger>
          <CollapsibleContent>
            <>
              <div className='flex justify-end border-t px-3 py-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => copyToClipboard(rawJsonText)}
                  disabled={!rawJsonText}
                >
                  {copiedText === rawJsonText ? (
                    <Check data-icon='inline-start' className='text-success' />
                  ) : (
                    <Copy data-icon='inline-start' />
                  )}
                  {t('Copy')}
                </Button>
              </div>
              <ScrollArea className='max-h-[50vh]'>
                <pre className='bg-muted/30 m-0 p-3 text-xs break-words whitespace-pre-wrap'>
                  {rawJsonText || '-'}
                </pre>
              </ScrollArea>
            </>
          </CollapsibleContent>
        </Collapsible>
      </div>
    </Dialog>
  )
}
