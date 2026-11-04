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
import type { TFunction } from 'i18next'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { getServerErrorMessage } from '@/lib/server-error-message'

import { getAllLogs, getUserLogs } from '../../api'
import type { UsageLog } from '../../data/schema'
import { parseLogOther } from '../../lib/format'
import {
  isSmartRouteScoringFailed,
  smartRouteTierText,
} from '../../lib/smart-route'
import type { SmartRouteLabels, SmartRouting } from '../../types'
import { DetailRow, DetailSection } from './log-detail-layout'

const DEFAULT_TIER_REASONS = new Set([
  'scorer_timeout',
  'scorer_error',
  'scorer_invalid',
])

function labelNames(t: TFunction): Record<string, string> {
  return {
    task_type: t('Task type'),
    reasoning: t('Reasoning depth'),
    workload: t('Workload'),
    risk: t('Risk of error'),
    domain: t('Work domain'),
    confidence: t('Confidence', { context: 'routing' }),
    feedback: t('User feedback'),
  }
}

function labelValues(t: TFunction): Record<string, Record<string, string>> {
  return {
    task_type: {
      chat_qa: t('Chat or simple Q&A'),
      lookup: t('Look up information'),
      rewrite: t('Rewrite or organize'),
      generate: t('Generate or edit content'),
      analyze: t('Analyze or troubleshoot'),
      plan: t('Plan or design'),
    },
    reasoning: {
      low: t('Low'),
      medium: t('Medium'),
      high: t('High'),
    },
    workload: {
      one_step: t('One step'),
      few_steps: t('A few steps'),
      many_steps: t('Many steps'),
    },
    risk: { low: t('Low'), high: t('High') },
    domain: {
      code: t('Code'),
      finance: t('Finance', { context: 'routing' }),
      operations: t('Operations', { context: 'routing' }),
      data_analysis: t('Data analysis'),
      other: t('Other'),
    },
    feedback: {
      satisfied: t('Satisfied'),
      dissatisfied: t('Dissatisfied'),
      neutral: t('Neutral'),
    },
  }
}

const LABEL_ORDER: Array<keyof SmartRouteLabels> = [
  'task_type',
  'reasoning',
  'workload',
  'risk',
  'domain',
  'confidence',
  'feedback',
]

function labelValueText(
  t: TFunction,
  key: keyof SmartRouteLabels,
  labels: SmartRouteLabels
): string {
  const value = labels[key]
  if (typeof value === 'number') return value.toFixed(2)
  return labelValues(t)[key]?.[value ?? ''] ?? String(value)
}

function smartRouteLabelsText(t: TFunction, labels: SmartRouteLabels): string {
  const names = labelNames(t)
  return LABEL_ORDER.filter((key) => labels[key] != null)
    .map((key) =>
      t('{{name}}: {{value}}', {
        name: names[key],
        value: labelValueText(t, key, labels),
      })
    )
    .join(' · ')
}

function smartRouteReasonText(
  t: TFunction,
  routing: SmartRouting,
  locale: string | undefined
): { text: string; danger: boolean } | null {
  const params = routing.reason_params ?? {}
  switch (routing.reason) {
    case 'downgrade_blocked':
      return {
        text: t(
          'History is {{tokens}} tokens, over the downgrade limit {{limit}}. Kept the previous {{tier}}',
          {
            tokens: formatNumber(params.history_tokens, locale),
            limit: formatNumber(params.limit, locale),
            tier: smartRouteTierText(t, params.kept_tier),
          }
        ),
        danger: false,
      }
    case 'advisor':
      return {
        text: t('The user was dissatisfied {{count}} messages in a row', {
          count: formatNumber(params.count, locale),
        }),
        danger: false,
      }
    case 'scorer_timeout':
      return {
        text: t('Scoring timed out ({{ms}} ms). Used the default tier', {
          ms: formatNumber(params.timeout_ms, locale),
        }),
        danger: true,
      }
    case 'scorer_error':
      return { text: t('Scoring failed. Used the default tier'), danger: true }
    case 'scorer_invalid':
      return {
        text: t('Invalid score result. Used the default tier'),
        danger: true,
      }
    case 'tool_continuation':
      return {
        text: t('Tool result continuation. Kept the tier of this turn'),
        danger: false,
      }
    case 'low_confidence':
      return {
        text: t('Scorer confidence is below the threshold'),
        danger: false,
      }
    case 'rule_simple':
      return { text: t('Matched the simple tier rule'), danger: false }
    case 'rule_normal':
      return { text: t('No special rule matched'), danger: false }
    case 'rule_hard': {
      const names = labelNames(t)
      const labels = routing.labels ?? {}
      const items = (params.matched ?? []).map(
        (key) =>
          `${names[key]}=${labelValueText(t, key as keyof SmartRouteLabels, labels)}`
      )
      return { text: items.join(t('、')), danger: false }
    }
    default:
      return null
  }
}

function useRelatedSmartRouteLog(props: {
  requestId: string
  createdAt: number
  role: string
  isAdmin: boolean
}) {
  const { t } = useTranslation()
  const [related, setRelated] = useState<UsageLog | null>(null)
  const [loading, setLoading] = useState(false)

  const open = async () => {
    if (loading) return
    setLoading(true)
    try {
      const fetchLogs = props.isAdmin ? getAllLogs : getUserLogs
      const res = await fetchLogs({
        request_id: props.requestId,
        start_timestamp: props.createdAt - 3600,
        end_timestamp: props.createdAt + 3600,
        p: 1,
        page_size: 20,
      })
      if (!res.success) {
        toast.error(getServerErrorMessage(res, t('Matching record not found')))
        return
      }
      const items = (res.data?.items ?? []) as UsageLog[]
      const found = items.find(
        (item) => parseLogOther(item.other)?.smart_routing?.role === props.role
      )
      if (!found) {
        toast.error(t('Matching record not found'))
        return
      }
      setRelated(found)
    } catch (error) {
      handleServerError(error)
    } finally {
      setLoading(false)
    }
  }

  return { related, loading, open, close: () => setRelated(null) }
}

export function SmartRouteSection(props: {
  log: UsageLog
  routing: SmartRouting
  isAdmin: boolean
  renderRelated: (log: UsageLog, onClose: () => void) => ReactNode
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const routing = props.routing
  const isScorer = routing.role === 'scorer'
  const counterpart = useRelatedSmartRouteLog({
    requestId: props.log.request_id,
    createdAt: props.log.created_at,
    role: isScorer ? 'main' : 'scorer',
    isAdmin: props.isAdmin,
  })

  if (isScorer) {
    return (
      <>
        <DetailSection label={t('Smart routing')}>
          <DetailRow label={t('Type')} value={t('Routing score')} />
          <DetailRow
            label={t('Virtual model')}
            value={routing.virtual_model}
            mono
          />
          <DetailRow
            label={t('Belongs to request')}
            value={
              <Button
                variant='link'
                size='sm'
                className='h-auto p-0 text-xs'
                disabled={counterpart.loading}
                onClick={counterpart.open}
              >
                {t('{{id}} ({{tier}} · {{model}})', {
                  id: props.log.request_id,
                  tier: smartRouteTierText(t, routing.main_tier),
                  model: routing.main_model,
                })}
              </Button>
            }
          />
        </DetailSection>
        {counterpart.related &&
          props.renderRelated(counterpart.related, counterpart.close)}
      </>
    )
  }

  const reason = smartRouteReasonText(t, routing, locale)
  const scoringFailed = isSmartRouteScoringFailed(routing)
  let tierText = smartRouteTierText(t, routing.tier)
  if (DEFAULT_TIER_REASONS.has(routing.reason ?? '')) {
    tierText = t('{{tier}} (default tier)', { tier: tierText })
  } else if (routing.reason === 'downgrade_blocked' && routing.scored_tier) {
    tierText = t('{{tier}} (scored: {{scored}})', {
      tier: tierText,
      scored: smartRouteTierText(t, routing.scored_tier),
    })
  }
  const showLabels = routing.labels && routing.reason !== 'tool_continuation'

  let scoreRecord = null
  if (routing.scorer?.billed === true) {
    scoreRecord = (
      <Button
        variant='link'
        size='sm'
        className='h-auto p-0 text-xs'
        disabled={counterpart.loading}
        onClick={counterpart.open}
      >
        {t('View this score')}
      </Button>
    )
  } else if (scoringFailed) {
    scoreRecord = t('No usage returned, not billed')
  }

  return (
    <>
      <DetailSection label={t('Smart routing')}>
        <DetailRow
          label={t('Virtual model')}
          value={routing.virtual_model}
          mono
        />
        <DetailRow label={t('Tier')} value={tierText} />
        {reason && (
          <DetailRow
            label={t('Tier reason')}
            value={
              <span className={reason.danger ? 'text-destructive' : undefined}>
                {reason.text}
              </span>
            }
          />
        )}
        {showLabels && (
          <DetailRow
            label={t('Score labels')}
            value={smartRouteLabelsText(t, routing.labels ?? {})}
          />
        )}
        {(routing.cleaned_reasoning ?? 0) > 0 && (
          <DetailRow
            label={t('Reasoning content')}
            value={t(
              'The previous turn used another channel. Cleaned {{count}} items',
              { count: routing.cleaned_reasoning }
            )}
          />
        )}
        {scoreRecord && (
          <DetailRow label={t('Score record')} value={scoreRecord} />
        )}
      </DetailSection>
      {counterpart.related &&
        props.renderRelated(counterpart.related, counterpart.close)}
    </>
  )
}
