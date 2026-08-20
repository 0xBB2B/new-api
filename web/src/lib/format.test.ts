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
import { afterEach, beforeEach, describe, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyConfig,
} from '@/stores/system-config-store'

import { parseQuotaFromDollars, quotaUnitsToDollars } from './format'

const initialConfig = useSystemConfigStore.getState().config

function setCurrency(overrides: Partial<CurrencyConfig>): void {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: { ...DEFAULT_CURRENCY_CONFIG, ...overrides },
    },
  }))
}

afterEach(() => {
  useSystemConfigStore.setState({ config: initialConfig })
})

describe('USD display', () => {
  beforeEach(() => {
    setCurrency({
      quotaDisplayType: 'USD',
      quotaPerUnit: 500000,
      usdExchangeRate: 1,
    })
  })

  test('parses dollar amounts into quota units', () => {
    expect(parseQuotaFromDollars(0.2)).toBe(100000)
    expect(parseQuotaFromDollars(2)).toBe(1000000)
  })

  test('converts quota units back into dollar amounts', () => {
    expect(quotaUnitsToDollars(100000)).toBe(0.2)
  })

  test('round-trips integer quota values', () => {
    for (const units of [0, 100000, 500000, 123457]) {
      expect(parseQuotaFromDollars(quotaUnitsToDollars(units))).toBe(units)
    }
  })
})

describe('currency display with exchange rate', () => {
  beforeEach(() => {
    setCurrency({
      quotaDisplayType: 'CNY',
      quotaPerUnit: 500000,
      usdExchangeRate: 7,
    })
  })

  test('parses local currency amount through exchange rate into quota units', () => {
    expect(parseQuotaFromDollars(1.4)).toBe(100000)
  })

  test('converts quota units back into local currency amount', () => {
    expect(Math.abs(quotaUnitsToDollars(100000) - 1.4) < 1e-9).toBeTruthy()
  })
})

describe('tokens display', () => {
  beforeEach(() => {
    setCurrency({
      quotaDisplayType: 'TOKENS',
      quotaPerUnit: 500000,
      usdExchangeRate: 1,
    })
  })

  test('parse and convert are identity operations', () => {
    expect(parseQuotaFromDollars(100000)).toBe(100000)
    expect(quotaUnitsToDollars(100000)).toBe(100000)
  })
})

describe('boundary inputs', () => {
  beforeEach(() => {
    setCurrency({
      quotaDisplayType: 'USD',
      quotaPerUnit: 500000,
      usdExchangeRate: 1,
    })
  })

  test('NaN amount parses to zero quota units', () => {
    expect(parseQuotaFromDollars(NaN)).toBe(0)
  })

  test('negative amount parses to negative quota units', () => {
    expect(parseQuotaFromDollars(-2)).toBe(-1000000)
  })
})
