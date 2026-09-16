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
  axes?: Array<{
    orient: string
    label?: { formatMethod?: AxisLabelFormatMethod }
  }>
  legends: {
    item?: { label?: { formatMethod?: AxisLabelFormatMethod } }
  }
  tooltip: {
    mark: { content: Array<{ key: TooltipKey }> }
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
  },
  {
    user_id: 2,
    username: 'bob',
    display_name: 'bob',
    created_at: 1_700_000_000,
    quota: 70,
  },
  { user_id: 9, username: 'ghost', created_at: 1_700_000_000, quota: 10 },
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
