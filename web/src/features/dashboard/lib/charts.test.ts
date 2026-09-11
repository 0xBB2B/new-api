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
import { describe, expect, test } from 'vitest'

import type { QuotaDataItem } from '../types'
import { processUserChartData } from './charts'

type TooltipKey = (datum: Record<string, unknown>) => unknown

interface UserChartSpecShape {
  data: Array<{ values: Array<Record<string, unknown>> }>
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
  test('keeps the username on the axis and shows the display name in tooltips', () => {
    const result = processUserChartData(rows, 'day')
    const rank = result.spec_user_rank as unknown as UserChartSpecShape
    const trend = result.spec_user_trend as unknown as UserChartSpecShape

    const rankValues = rank.data[0].values
    expect(rankValues.map((v) => v.User)).toEqual(['oidc_1', 'bob', 'ghost'])
    expect(rankValues.map((v) => rank.tooltip.mark.content[0].key(v))).toEqual([
      'oidc_1 (Alice Liddell)',
      'bob',
      'ghost',
    ])

    const aliceTrend = trend.data[0].values.find((v) => v.User === 'oidc_1')
    expect(aliceTrend).toBeDefined()
    expect(trend.tooltip.mark.content[0].key(aliceTrend ?? {})).toBe(
      'oidc_1 (Alice Liddell)'
    )
    expect(trend.tooltip.dimension?.content[0].key(aliceTrend ?? {})).toBe(
      'oidc_1 (Alice Liddell)'
    )
  })
})
