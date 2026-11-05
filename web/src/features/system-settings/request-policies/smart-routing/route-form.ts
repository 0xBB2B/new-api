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
import { z } from 'zod'

import { SMART_EFFORTS } from './capability'
import type { SmartRoute } from './types'

export type RouteFormValues = SmartRoute

export function createRouteFormSchema(t: TFunction) {
  const range = (min: number, max: number) =>
    t('Enter a value between {{min}} and {{max}}', { min, max })
  const integerRange = (min: number, max: number) =>
    z
      .number(range(min, max))
      .int(range(min, max))
      .min(min, range(min, max))
      .max(max, range(min, max))
  const model = z.string().min(1, t('Select a model'))
  const groups = z.array(z.string()).min(1, t('Select at least one group'))
  const effort = z.enum(['', ...SMART_EFFORTS])
  const tier = z
    .object({
      model,
      groups,
      effort_mode: z.enum(['', 'fixed', 'follow']),
      effort,
      effort_min: effort,
      effort_max: effort,
    })
    .superRefine((value, ctx) => {
      const issue = (path: string, message: string) =>
        ctx.addIssue({ code: 'custom', path: [path], message })
      const invalid = t('Select a valid reasoning effort')
      const { effort_mode: mode } = value
      if (mode !== 'fixed' && value.effort) issue('effort', invalid)
      if (mode !== 'follow' && value.effort_min) issue('effort_min', invalid)
      if (mode !== 'follow' && value.effort_max) issue('effort_max', invalid)
      if (mode === 'fixed' && !value.effort) issue('effort', invalid)
      if (mode !== 'follow') return
      if (!value.effort_min) issue('effort_min', invalid)
      if (!value.effort_max) issue('effort_max', invalid)
      if (
        value.effort_min &&
        value.effort_max &&
        SMART_EFFORTS.indexOf(value.effort_min) >
          SMART_EFFORTS.indexOf(value.effort_max)
      ) {
        issue(
          'effort_max',
          t('The minimum effort cannot be higher than the maximum')
        )
      }
    })

  return z.object({
    name: z.string().trim().min(1, t('Enter a name')),
    endpoints: z
      .array(z.enum(['openai-response', 'openai', 'anthropic']))
      .min(1, t('Select at least one endpoint')),
    tiers: z.object({ simple: tier, normal: tier, hard: tier }),
    default_tier: z.enum(['simple', 'normal', 'hard']),
    scorer: z.object({
      model,
      groups,
      timeout_ms: integerRange(100, 30000),
    }),
    advisor: z.object({
      model,
      groups,
      after_dissatisfied: integerRange(1, 10),
    }),
    downgrade_max_history_tokens: z
      .number(t('Enter an integer of 0 or more'))
      .int(t('Enter an integer of 0 or more'))
      .min(0, t('Enter an integer of 0 or more')),
    confidence_threshold: z
      .number(range(0, 1))
      .min(0, range(0, 1))
      .max(1, range(0, 1)),
    simple_max_message_chars: integerRange(1, 100000),
  })
}

export function emptyRouteFormValues(): RouteFormValues {
  const tier = () => ({
    model: '',
    groups: [],
    effort_mode: '' as const,
    effort: '' as const,
    effort_min: '' as const,
    effort_max: '' as const,
  })
  return {
    name: '',
    endpoints: ['openai-response'],
    tiers: { simple: tier(), normal: tier(), hard: tier() },
    default_tier: 'normal',
    scorer: { model: '', groups: [], timeout_ms: 2000 },
    advisor: { model: '', groups: [], after_dissatisfied: 2 },
    downgrade_max_history_tokens: 20000,
    confidence_threshold: 0.6,
    simple_max_message_chars: 2000,
  }
}

export function formValuesToRoute(values: RouteFormValues): SmartRoute {
  return { ...values, name: values.name.trim() }
}

export function routeToFormValues(route: SmartRoute): RouteFormValues {
  const defaults = emptyRouteFormValues()
  return {
    ...defaults,
    ...route,
    // 后端把缺失的数字字段当 0 运行，这两个字段 0 合法，补默认值会让一次原样保存改变路由行为。
    downgrade_max_history_tokens: route.downgrade_max_history_tokens ?? 0,
    confidence_threshold: route.confidence_threshold ?? 0,
    tiers: {
      simple: { ...defaults.tiers.simple, ...route.tiers?.simple },
      normal: { ...defaults.tiers.normal, ...route.tiers?.normal },
      hard: { ...defaults.tiers.hard, ...route.tiers?.hard },
    },
    scorer: { ...defaults.scorer, ...route.scorer },
    advisor: { ...defaults.advisor, ...route.advisor },
  }
}
