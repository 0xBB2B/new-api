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
import { AlertTriangle, Check } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'
import { createServerError } from '@/lib/server-error-message'

import { resetClaudeLimit, type ClaudeUsageResponse } from '../../api'
import {
  resolveClaudeLimitResets,
  type ClaudeLimitResetRequest,
} from '../../lib/subscription-usage'

type ClaudeLimitResetCardProps = {
  channelId?: number
  response: ClaudeUsageResponse | null
  isRefreshing?: boolean
  onRefresh?: () => void | Promise<void>
}

type ResetKind = 'full' | 'five_hour'

type ResetRowProps = {
  title: string
  description: ReactNode
  canReset?: boolean
  showButton: boolean
  disabled: boolean
  isResetting: boolean
  onReset: () => void
}

function ResetRow(props: ResetRowProps) {
  const { t } = useTranslation()
  return (
    <div className='flex items-center justify-between gap-3'>
      <div className='min-w-0'>
        <div className='text-sm font-medium'>{props.title}</div>
        <div className='text-muted-foreground mt-0.5 text-xs'>
          {props.description}
        </div>
      </div>
      {props.showButton ? (
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={!props.canReset || props.disabled}
          onClick={props.onReset}
        >
          {props.isResetting ? t('Resetting...') : t('Reset for free')}
        </Button>
      ) : null}
    </div>
  )
}

function formatMinute(timestamp: number): string {
  return formatTimestampToDate(timestamp).slice(0, 16)
}

export function ClaudeLimitResetCard(props: ClaudeLimitResetCardProps) {
  const { t } = useTranslation()
  const [confirmKind, setConfirmKind] = useState<ResetKind | null>(null)
  const [isResetting, setIsResetting] = useState(false)
  const [successMessage, setSuccessMessage] = useState('')
  const [errorMessage, setErrorMessage] = useState('')

  const resets = props.response
    ? resolveClaudeLimitResets(props.response, Math.floor(Date.now() / 1000))
    : null
  const noneText = t('None right now. It shows up here when you get one')

  const clearName = (name: string) => {
    if (name === 'five_hour') return t('5-Hour Window')
    if (name === 'seven_day') return t('Weekly Window')
    return name
  }

  const requestFor = (kind: ResetKind): ClaudeLimitResetRequest | null => {
    if (resets?.state !== 'ready') return null
    if (kind === 'full') {
      return resets.full.available && resets.full.canReset
        ? resets.full.request
        : null
    }
    return resets.fiveHour.canReset ? resets.fiveHour.request : null
  }

  const handleConfirm = async () => {
    const request = confirmKind ? requestFor(confirmKind) : null
    if (!props.channelId || !request || isResetting) return

    setIsResetting(true)
    setSuccessMessage('')
    setErrorMessage('')
    try {
      const res = await resetClaudeLimit(props.channelId, request)
      if (!res.success) {
        throw createServerError(res, t('Failed to reset usage'))
      }
      setSuccessMessage(t('Reset completed'))
      setConfirmKind(null)
      await Promise.resolve(props.onRefresh?.())
    } catch (error) {
      setConfirmKind(null)
      setErrorMessage(
        error instanceof Error ? error.message : t('Failed to reset usage')
      )
    } finally {
      setIsResetting(false)
    }
  }

  let body: ReactNode = null
  if (props.isRefreshing) {
    body = (
      <div className='flex flex-col gap-3'>
        <Skeleton className='h-9 w-full' />
        <Skeleton className='h-9 w-full' />
      </div>
    )
  } else if (resets?.state === 'client_version') {
    body = (
      <div className='text-muted-foreground text-xs'>
        {t(
          'Cannot get the latest Claude Code version, limit resets are unavailable for now'
        )}
      </div>
    )
  } else if (resets?.state === 'not_returned') {
    body = (
      <div className='text-muted-foreground text-xs'>
        {t('Upstream did not return limit reset info')}
      </div>
    )
  } else if (resets?.state === 'ready') {
    const full = resets.full
    const fiveHour = resets.fiveHour
    const canClick = Boolean(props.channelId) && !isResetting

    let fullDescription: ReactNode = noneText
    if (full.available) {
      fullDescription = (
        <>
          <div>
            {t('{{label}} · {{left}}/{{total}} left · Expires {{time}}', {
              label: full.label,
              left: full.resetsLeft,
              total: full.resetsTotal,
              time: formatMinute(full.endsAt),
            })}
          </div>
          <div>
            {t('Clears: ')}
            {full.clears.map(clearName).join(', ')}
          </div>
          {full.coolingDown ? (
            <div>
              {t('Cooling down, available after {{time}}', {
                time: formatMinute(full.cooldownUntil),
              })}
            </div>
          ) : null}
        </>
      )
    }

    let fiveHourDescription: ReactNode = noneText
    if (fiveHour.canReset) {
      fiveHourDescription = t(
        'Available this week · Clears only the 5-hour window, weekly limit unchanged'
      )
    } else if (fiveHour.reasonKind === 'not_at_wall') {
      fiveHourDescription = t('Shows up when the 5-hour limit is used up')
    } else if (fiveHour.reasonKind === 'client_identity') {
      fiveHourDescription = t('Client identity check failed')
    } else if (fiveHour.reasonKind === 'raw') {
      fiveHourDescription = fiveHour.rawReason
    } else if (fiveHour.reasonKind === 'next_available') {
      fiveHourDescription = t('Available after {{time}}', {
        time: formatMinute(fiveHour.nextAvailableAt),
      })
    }

    body = (
      <div className='flex flex-col gap-3'>
        <ResetRow
          title={t('Full reset')}
          description={fullDescription}
          showButton={full.available}
          canReset={full.available && full.canReset}
          disabled={!canClick}
          isResetting={isResetting && confirmKind === 'full'}
          onReset={() => setConfirmKind('full')}
        />
        <ResetRow
          title={t('5-hour reset')}
          description={fiveHourDescription}
          showButton={fiveHour.canReset}
          canReset={fiveHour.canReset}
          disabled={!canClick}
          isResetting={isResetting && confirmKind === 'five_hour'}
          onReset={() => setConfirmKind('five_hour')}
        />
      </div>
    )
  }

  if (!body && !successMessage && !errorMessage) {
    return null
  }

  const confirmFull = confirmKind === 'full'

  return (
    <Card size='sm' className='bg-muted/30 gap-0 py-0'>
      <CardHeader className='p-4 pb-2'>
        <CardTitle className='text-muted-foreground text-xs font-medium'>
          {t('Limit resets')}
        </CardTitle>
      </CardHeader>
      <CardContent className='flex flex-col gap-3 p-4 pt-0'>
        {successMessage ? (
          <Alert className='border-success/40 bg-success/10 text-success'>
            <Check />
            <AlertTitle>{successMessage}</AlertTitle>
          </Alert>
        ) : null}
        {errorMessage ? (
          <Alert variant='destructive'>
            <AlertTriangle />
            <AlertTitle>{t('Reset failed')}</AlertTitle>
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        ) : null}
        {body}
      </CardContent>
      <ConfirmDialog
        open={confirmKind !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmKind(null)
        }}
        title={
          confirmFull ? t('Confirm full reset') : t('Confirm 5-hour reset')
        }
        desc={
          confirmFull ? (
            <div className='flex flex-col gap-2'>
              <p>
                {t(
                  'This clears the windows listed below right away. Once used, this reset is gone.'
                )}
              </p>
              {resets?.state === 'ready' && resets.full.available ? (
                <div className='bg-muted/50 rounded-lg border px-3 py-2 text-xs'>
                  {t('Clears: ')}
                  {resets.full.clears.map(clearName).join(', ')}
                </div>
              ) : null}
            </div>
          ) : (
            t(
              'This clears only the 5-hour window. The weekly limit stays unchanged.'
            )
          )
        }
        destructive
        isLoading={isResetting}
        cancelBtnText={t('Cancel')}
        confirmText={isResetting ? t('Resetting...') : t('Reset for free')}
        handleConfirm={handleConfirm}
      />
    </Card>
  )
}
