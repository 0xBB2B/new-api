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
import i18next from 'i18next'
import { describe, expect, test } from 'vitest'

import type { QuotaDataItem } from '../types'
import { processUserChartData } from './charts'

type TooltipKey = (datum: Record<string, unknown>) => unknown
type AxisLabelFormatMethod = (value: string) => string

interface UserChartSpecShape {
  data: Array<{ values: Array<Record<string, unknown>> }>
  title: { subtext: string }
  label?: { formatMethod?: (value: number) => string }
  axes?: Array<{
    orient: string
    max?: number
    label?: { formatMethod?: (value: string | number) => string }
  }>
  legends: {
    item?: { label?: { formatMethod?: AxisLabelFormatMethod } }
  }
  tooltip: {
    mark: { content: Array<{ key: TooltipKey; value: TooltipKey }> }
    dimension?: { content: Array<{ key: TooltipKey }> }
  }
}

const rows: QuotaDataItem[] = [
  {
    user_id: 1,
    username: 'oidc_1',
    display_name: 'Alice Liddell',
    created_at: 1_700_000_000,
    quota: 150,
    token_used: 1_200,
  },
  {
    user_id: 2,
    username: 'bob',
    display_name: 'bob',
    created_at: 1_700_000_000,
    quota: 70,
    token_used: 5_000,
  },
  {
    user_id: 9,
    username: 'ghost',
    created_at: 1_700_000_000,
    quota: 10,
    token_used: 300,
  },
]

describe('processUserChartData', () => {
  test('keeps the username as the grouping key and formats tooltip labels as name · username · ID', () => {
    const result = processUserChartData(rows, 'day')
    const rank = result.spec_user_rank as unknown as UserChartSpecShape
    const trend = result.spec_user_trend as unknown as UserChartSpecShape

    const rankValues = rank.data[0].values
    expect(rankValues.map((v) => v.User)).toEqual(['oidc_1', 'bob', 'ghost'])
    expect(rankValues.map((v) => rank.tooltip.mark.content[0].key(v))).toEqual([
      'Alice Liddell · oidc_1 · ID:1',
      'bob · ID:2',
      'ghost · ID:9',
    ])
    expect(
      rankValues.map((v) => rank.tooltip.dimension?.content[0].key(v))
    ).toEqual(['Alice Liddell · oidc_1 · ID:1', 'bob · ID:2', 'ghost · ID:9'])

    const trendValues = trend.data[0].values
    const aliceTrend = trendValues.find((v) => v.User === 'oidc_1')
    expect(trend.tooltip.mark.content[0].key(aliceTrend ?? {})).toBe(
      'Alice Liddell · oidc_1 · ID:1'
    )
    expect(trend.tooltip.dimension?.content[0].key(aliceTrend ?? {})).toBe(
      'Alice Liddell · oidc_1 · ID:1'
    )
  })

  test('falls back to the localized id name when a row has no username', () => {
    const result = processUserChartData(
      [{ user_id: 42, username: '', created_at: 1_700_000_000, quota: 5 }],
      'day',
      i18next.t
    )
    const rank = result.spec_user_rank as unknown as UserChartSpecShape

    const rankValues = rank.data[0].values
    expect(rankValues.map((v) => v.User)).toEqual(['unknown'])
    expect(rankValues.map((v) => rank.tooltip.mark.content[0].key(v))).toEqual([
      'User 42 · ID:42',
    ])
    const leftAxis = rank.axes?.find((axis) => axis.orient === 'left')
    expect(leftAxis?.label?.formatMethod?.('unknown')).toBe('User 42')
  })

  test('ranks and formats by token_used when the metric is tokens', () => {
    const result = processUserChartData(rows, 'day', undefined, 10, 'tokens')
    const rank = result.spec_user_rank as unknown as UserChartSpecShape
    const trend = result.spec_user_trend as unknown as UserChartSpecShape

    const rankValues = rank.data[0].values
    expect(rankValues.map((v) => v.User)).toEqual(['bob', 'oidc_1', 'ghost'])
    expect(rankValues.map((v) => v.rawValue)).toEqual([5_000, 1_200, 300])
    expect(rank.tooltip.mark.content[0].value(rankValues[0])).toBe('5,000')

    const bobTrend = trend.data[0].values.find((v) => v.User === 'bob')
    expect(bobTrend?.rawValue).toBe(5_000)
    expect(trend.tooltip.mark.content[0].value(bobTrend ?? {})).toBe('5,000')
  })

  test('abbreviates token bar labels, axis ticks and totals with K/M/B units', () => {
    const big: QuotaDataItem[] = [
      {
        user_id: 1,
        username: 'a',
        created_at: 1_700_000_000,
        token_used: 685_000,
      },
      {
        user_id: 2,
        username: 'b',
        created_at: 1_700_000_000,
        token_used: 1_375_000,
      },
      {
        user_id: 3,
        username: 'c',
        created_at: 1_700_000_000,
        token_used: 2_100_000_000,
      },
    ]
    const result = processUserChartData(big, 'day', undefined, 10, 'tokens')
    const rank = result.spec_user_rank as unknown as UserChartSpecShape
    const trend = result.spec_user_trend as unknown as UserChartSpecShape

    expect(rank.label?.formatMethod?.(685_000)).toBe('685K')
    expect(rank.label?.formatMethod?.(1_375_000)).toBe('1.4M')
    expect(rank.label?.formatMethod?.(2_100_000_000)).toBe('2.1B')
    expect(rank.title.subtext).toContain('2.1B')
    const leftAxis = trend.axes?.find((axis) => axis.orient === 'left')
    expect(leftAxis?.label?.formatMethod?.(150_000)).toBe('150K')
    expect(rank.tooltip.mark.content[0].value(rank.data[0].values[0])).toBe(
      '2,100,000,000'
    )
  })

  test('reserves headroom past the longest bar so its outside label is not pushed inside', () => {
    const result = processUserChartData(rows, 'day', undefined, 10, 'tokens')
    const rank = result.spec_user_rank as unknown as UserChartSpecShape
    const bottomAxis = rank.axes?.find((axis) => axis.orient === 'bottom')

    expect(bottomAxis?.max).toBeCloseTo(5_750)
  })

  test('leaves the value axis domain to VChart when every user total is zero', () => {
    const result = processUserChartData(
      [{ user_id: 1, username: 'a', created_at: 1_700_000_000, quota: 0 }],
      'day'
    )
    const rank = result.spec_user_rank as unknown as UserChartSpecShape
    const bottomAxis = rank.axes?.find((axis) => axis.orient === 'bottom')

    expect(bottomAxis?.max).toBeUndefined()
  })

  test('resolves the band axis and legend labels from the username to the display name', () => {
    const result = processUserChartData(rows, 'day')
    const rank = result.spec_user_rank as unknown as UserChartSpecShape
    const trend = result.spec_user_trend as unknown as UserChartSpecShape
    const leftAxis = rank.axes?.find((axis) => axis.orient === 'left')

    expect(leftAxis?.label?.formatMethod?.('oidc_1')).toBe('Alice Liddell')
    expect(leftAxis?.label?.formatMethod?.('ghost')).toBe('ghost')
    expect(trend.legends.item?.label?.formatMethod?.('oidc_1')).toBe(
      'Alice Liddell'
    )
  })
})
