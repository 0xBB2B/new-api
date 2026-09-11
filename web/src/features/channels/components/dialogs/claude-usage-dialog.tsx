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
import { ScrollArea } from '@/components/ui/scroll-area'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { formatTimestampToDate } from '@/lib/format'

import type { ClaudeUsageResponse } from '../../api'
import { resolveRateLimitWindows } from '../../lib/subscription-usage'
import { RateLimitWindowGrid } from './rate-limit-window-grid'

type ClaudeUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelName?: string
  channelId?: number
  response: ClaudeUsageResponse | null
  onRefresh?: () => void | Promise<void>
  isRefreshing?: boolean
}

export function ClaudeUsageDialog({
  open,
  onOpenChange,
  channelName,
  channelId,
  response,
  onRefresh,
  isRefreshing,
}: ClaudeUsageDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const [showRawJson, setShowRawJson] = useState(false)

  const usage = response?.usage ?? null
  const { fiveHourWindow, weeklyWindow } = resolveRateLimitWindows(
    usage ? { plan_type: usage.plan_type, rate_limit: usage } : null
  )
  const planType = usage?.plan_type?.trim()
  const updatedAt = Number(usage?.updated_at)
  const errorMessage =
    response?.success === false
      ? response?.message?.trim() || t('Failed to fetch usage')
      : ''

  const rawJsonText = useMemo(() => {
    if (!response) {
      return ''
    }
    try {
      return JSON.stringify(
        {
          success: response.success,
          message: response.message,
          upstream_status: response.upstream_status,
          data: response.data,
        },
        null,
        2
      )
    } catch {
      return String(response?.data ?? '')
    }
  }, [response])

  const handleDialogOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setShowRawJson(false)
    }
    onOpenChange(nextOpen)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleDialogOpenChange}
      title={t('Claude Account & Usage')}
      contentClassName='sm:max-w-[720px]'
      contentHeight='auto'
      bodyClassName='flex flex-col gap-4'
      footer={
        <Button
          type='button'
          variant='outline'
          onClick={() => handleDialogOpenChange(false)}
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
              {t('Subscription')}
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
              <StatusBadge
                label={planType ? planType.toUpperCase() : t('Unknown')}
                variant={planType ? 'purple' : 'neutral'}
                copyable={false}
              />
              {usage?.limit_reached ? (
                <StatusBadge
                  label={t('Limit reached')}
                  variant='danger'
                  copyable={false}
                />
              ) : null}
              <StatusBadge
                label={`HTTP ${response?.upstream_status ?? '-'}`}
                variant='neutral'
                copyable={false}
              />
              <StatusBadge
                label={`${channelName ?? '-'}${channelId ? ` (#${channelId})` : ''}`}
                variant='neutral'
                copyable={false}
              />
            </div>
            {Number.isFinite(updatedAt) && updatedAt > 0 ? (
              <div className='text-muted-foreground mt-3 text-xs'>
                {t('Updated at:')} {formatTimestampToDate(updatedAt)}
              </div>
            ) : null}
          </CardContent>
        </Card>

        <RateLimitWindowGrid
          fiveHourWindow={fiveHourWindow}
          weeklyWindow={weeklyWindow}
        />

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
