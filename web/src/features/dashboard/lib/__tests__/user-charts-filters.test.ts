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
import { beforeEach, describe, expect, it } from 'vitest'

import { buildDefaultUserChartsFilters, saveGranularity } from '../filters'

beforeEach(() => {
  window.localStorage.clear()
})

describe('buildDefaultUserChartsFilters', () => {
  it('defaults to the Token metric and top 10 users when no granularity is saved', () => {
    const filters = buildDefaultUserChartsFilters()

    expect(filters.metric).toBe('tokens')
    expect(filters.topUserLimit).toBe(10)
    expect(filters.timeGranularity).toBe('hour')
    expect(filters.selectedRange).toBe(1)
  })

  it('uses the saved granularity and its default range while keeping the Token metric', () => {
    saveGranularity('week')

    const filters = buildDefaultUserChartsFilters()

    expect(filters.timeGranularity).toBe('week')
    expect(filters.selectedRange).toBe(29)
    expect(filters.metric).toBe('tokens')
  })
})
