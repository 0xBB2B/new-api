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

import type { UserUtilization } from '../../types'
import { buildUtilizationView } from '../utilization'

describe('buildUtilizationView', () => {
  test('典型数据：卡片与四段比例条', () => {
    const data: UserUtilization = {
      total_users: 36,
      deep_users: 13,
      frequent_users: 21,
      moderate_users: 13,
      inactive_users: 2,
    }
    const view = buildUtilizationView(data)

    expect(view.total).toBe(36)
    expect(view.cards.deep).toEqual({ count: 13, percent: '36%' })
    expect(view.cards.frequent).toEqual({ count: 21, percent: '58%' })
    expect(view.cards.inactive).toEqual({ count: 2, percent: '6%' })
    expect(view.segments.map((s) => s.key)).toEqual([
      'deep',
      'frequent',
      'moderate',
      'inactive',
    ])
    expect(view.segments.map((s) => s.count)).toEqual([13, 8, 13, 2])
    expect(view.segments.map((s) => s.percent)).toEqual([
      '36%',
      '22%',
      '36%',
      '6%',
    ])
  })

  test('total 为 0：占比全为破折号，仍有四段', () => {
    const view = buildUtilizationView({
      total_users: 0,
      deep_users: 0,
      frequent_users: 0,
      moderate_users: 0,
      inactive_users: 0,
    })

    expect(view.total).toBe(0)
    expect(view.cards.deep).toEqual({ count: 0, percent: '—' })
    expect(view.cards.frequent).toEqual({ count: 0, percent: '—' })
    expect(view.cards.inactive).toEqual({ count: 0, percent: '—' })
    expect(view.segments).toHaveLength(4)
    expect(view.segments.map((s) => s.count)).toEqual([0, 0, 0, 0])
    expect(view.segments.map((s) => s.percent)).toEqual(['—', '—', '—', '—'])
  })

  test('frequent 等于 deep：经常使用段为 0 且占比 0%', () => {
    const view = buildUtilizationView({
      total_users: 3,
      deep_users: 1,
      frequent_users: 1,
      moderate_users: 1,
      inactive_users: 1,
    })

    expect(view.segments[1]).toEqual({
      key: 'frequent',
      count: 0,
      percent: '0%',
    })
    expect(view.cards.frequent).toEqual({ count: 1, percent: '33%' })
  })

  test('占比 .5 向上进位', () => {
    const view = buildUtilizationView({
      total_users: 8,
      deep_users: 1,
      frequent_users: 1,
      moderate_users: 0,
      inactive_users: 7,
    })

    expect(view.cards.deep.percent).toBe('13%')
    expect(view.cards.inactive.percent).toBe('88%')
  })

  test('占比 .5 在除法有浮点误差时仍向上进位', () => {
    const view = buildUtilizationView({
      total_users: 200,
      deep_users: 29,
      frequent_users: 35,
      moderate_users: 0,
      inactive_users: 165,
    })

    expect(view.cards.deep.percent).toBe('15%')
    expect(view.cards.frequent.percent).toBe('18%')
  })

  test('frequent 小于 deep 的异常数据：经常使用段按 0 处理', () => {
    const view = buildUtilizationView({
      total_users: 2,
      deep_users: 2,
      frequent_users: 1,
      moderate_users: 0,
      inactive_users: 0,
    })

    expect(view.segments[1].count).toBe(0)
  })
})
