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
import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useStatus } from '@/hooks/use-status'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import { getUserUtilization } from '../../api'
import { buildUtilizationView } from '../../lib'
import type { UtilizationSegmentKey } from '../../lib/utilization'

const SEGMENT_META: Record<
  UtilizationSegmentKey,
  { nameKey: string; hintKey: string; colorClass: string }
> = {
  deep: {
    nameKey: 'Deep usage',
    hintKey: 'Over 100M tokens in the last 7 days',
    colorClass: 'bg-utilization-deep',
  },
  frequent: {
    nameKey: 'Frequent usage (under 100M)',
    hintKey: 'Over 1M but at most 100M tokens in the last 7 days',
    colorClass: 'bg-utilization-frequent',
  },
  moderate: {
    nameKey: 'Over 1M in the last 30 days',
    hintKey: 'At most 1M in the last 7 days, over 1M in the last 30 days',
    colorClass: 'bg-utilization-moderate',
  },
  inactive: {
    nameKey: 'Inactive',
    hintKey: 'At most 1M tokens in the last 30 days, including unused',
    colorClass: 'bg-utilization-inactive',
  },
}

const CARD_CLASS = 'min-h-44 rounded-2xl border p-6'

export function UtilizationOverview() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const { status } = useStatus()
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['dashboard', 'user-utilization'],
    queryFn: async () => requireServerSuccess(await getUserUtilization()),
    staleTime: 60_000,
  })

  const exportOff = status?.enable_data_export === false
  const notice = exportOff ? (
    <Alert className='border-warning/45 bg-warning/10 *:[svg]:text-warning mb-4 rounded-xl'>
      <AlertTriangle aria-hidden='true' />
      <AlertTitle>
        {t('Data dashboard is off, statistics are incomplete')}
      </AlertTitle>
      <AlertDescription>
        {t(
          'Utilization is calculated from usage recorded by the data dashboard. Calls made while it is off are not recorded, so those users count as inactive. Turn it on in System Settings → Console Content → Data Dashboard.'
        )}
      </AlertDescription>
    </Alert>
  ) : null
  const scope = (
    <p className='text-muted-foreground mb-3 text-sm'>
      {t(
        'Only enabled regular users are counted, excluding admins and Root. The last 7 and 30 days are counted back from now.'
      )}
    </p>
  )

  if (isLoading) {
    return (
      <div>
        {notice}
        {scope}
        <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          {Array.from({ length: 4 }, (_, i) => (
            <div key={i} className={CARD_CLASS}>
              <Skeleton className='h-14 w-24' />
              <Skeleton className='mt-4 h-5 w-36' />
              <Skeleton className='mt-3 h-4 w-44' />
            </div>
          ))}
        </div>
        <Skeleton className='mt-7 h-7 w-full rounded-full' />
        <Skeleton className='mt-3.5 h-4 w-3/5' />
      </div>
    )
  }

  if (isError || !data) {
    return (
      <div>
        {notice}
        {scope}
        <ErrorState
          title={t('Failed to load utilization data')}
          onRetry={() => void refetch()}
        />
      </div>
    )
  }

  const view = buildUtilizationView(data.data)
  const cardTitle = (nameKey: string, percent: string) => (
    <>
      {t(nameKey)}
      <span className='text-muted-foreground mx-2'>·</span>
      {t('{{percent}} of users', { percent })}
    </>
  )

  return (
    <div>
      {notice}
      {scope}
      <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4'>
        <div className={cn(CARD_CLASS, 'bg-card')}>
          <div className='text-4xl leading-none font-extrabold tracking-tight tabular-nums xl:text-[56px]'>
            {formatNumber(view.total, locale)}
          </div>
          <div className='mt-4 text-lg font-bold'>{t('Enabled users')}</div>
          <div className='text-muted-foreground mt-2 text-sm'>
            {t('Enabled regular users')}
          </div>
        </div>
        <div
          className={cn(
            CARD_CLASS,
            'bg-utilization-highlight border-transparent text-neutral-50 dark:border-utilization-highlight-number/45'
          )}
        >
          <div className='text-utilization-highlight-number text-4xl leading-none font-extrabold tracking-tight tabular-nums xl:text-[56px]'>
            {formatNumber(view.cards.deep.count, locale)}
          </div>
          <div className='mt-4 text-lg font-bold [&>span]:text-slate-300'>
            {cardTitle('Deep usage', view.cards.deep.percent)}
          </div>
          <div className='mt-2 text-sm text-slate-300'>
            {t(SEGMENT_META.deep.hintKey)}
          </div>
        </div>
        <div className={cn(CARD_CLASS, 'bg-card')}>
          <div className='text-4xl leading-none font-extrabold tracking-tight tabular-nums xl:text-[56px]'>
            {formatNumber(view.cards.frequent.count, locale)}
          </div>
          <div className='mt-4 text-lg font-bold'>
            {cardTitle('Frequent usage', view.cards.frequent.percent)}
          </div>
          <div className='text-muted-foreground mt-2 text-sm'>
            {t('Over 1M tokens in the last 7 days')}
          </div>
        </div>
        <div className='border-muted-foreground/40 min-h-44 rounded-2xl border-[1.5px] border-dashed bg-transparent p-6'>
          <div className='text-utilization-inactive text-4xl leading-none font-extrabold tracking-tight tabular-nums xl:text-[56px]'>
            {formatNumber(view.cards.inactive.count, locale)}
          </div>
          <div className='mt-4 text-lg font-bold'>
            {cardTitle('Inactive', view.cards.inactive.percent)}
          </div>
          <div className='text-muted-foreground mt-2 text-sm'>
            {t('At most 1M tokens in the last 30 days')}
          </div>
        </div>
      </div>

      <div className='mt-7'>
        {view.total > 0 ? (
          <div
            role='img'
            aria-label={view.segments
              .map(
                (s) =>
                  `${t(SEGMENT_META[s.key].nameKey)} ${t('{{count}} users · {{percent}}', { count: formatNumber(s.count, locale), percent: s.percent })}`
              )
              .join(', ')}
            className='bg-muted flex h-7 gap-0.5 overflow-hidden rounded-full'
          >
            {view.segments
              .filter((s) => s.count > 0)
              .map((s) => (
                <Tooltip key={s.key}>
                  <TooltipTrigger
                    render={
                      <div
                        className={cn('h-full', SEGMENT_META[s.key].colorClass)}
                        style={{ flex: `${s.count} 1 0` }}
                      />
                    }
                  />
                  <TooltipContent className='flex-col items-start gap-1'>
                    <span className='flex items-center gap-1.5 font-semibold'>
                      <span
                        className={cn(
                          'size-2.5 rounded-[3px]',
                          SEGMENT_META[s.key].colorClass
                        )}
                      />
                      {t(SEGMENT_META[s.key].nameKey)}
                    </span>
                    <span>
                      {t('{{count}} users · {{percent}}', {
                        count: formatNumber(s.count, locale),
                        percent: s.percent,
                      })}
                    </span>
                    <span>{t(SEGMENT_META[s.key].hintKey)}</span>
                  </TooltipContent>
                </Tooltip>
              ))}
          </div>
        ) : (
          <div className='bg-muted text-muted-foreground flex h-7 items-center justify-center rounded-full text-xs'>
            {t('No regular users yet')}
          </div>
        )}
        <div className='mt-3.5 flex flex-wrap gap-x-5 gap-y-2'>
          {view.segments.map((s) => (
            <span key={s.key} className='inline-flex items-center gap-2'>
              <span
                className={cn(
                  'size-3.5 shrink-0 rounded',
                  SEGMENT_META[s.key].colorClass
                )}
              />
              {t(SEGMENT_META[s.key].nameKey)}
              <span className='text-muted-foreground tabular-nums'>
                {t('{{count}} users · {{percent}}', {
                  count: formatNumber(s.count, locale),
                  percent: s.percent,
                })}
              </span>
            </span>
          ))}
        </div>
      </div>
    </div>
  )
}
