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
import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import {
  parseSubscriptionUsageSnapshot,
  resetCountdownSeconds,
  resolveRateLimitWindows,
  windowLabel,
  type CodexRateLimitWindow,
} from '../subscription-usage'

describe('parseSubscriptionUsageSnapshot', () => {
  test('returns null when other_info is empty, malformed, or lacks subscription_usage', () => {
    assert.equal(parseSubscriptionUsageSnapshot(''), null)
    assert.equal(parseSubscriptionUsageSnapshot(undefined), null)
    assert.equal(parseSubscriptionUsageSnapshot('{oops'), null)
    assert.equal(parseSubscriptionUsageSnapshot('{"status_reason":"x"}'), null)
    assert.equal(parseSubscriptionUsageSnapshot('{"subscription_usage":"nope"}'), null)
  })

  test('reads the snapshot next to other keys', () => {
    const snapshot = parseSubscriptionUsageSnapshot(
      JSON.stringify({
        status_reason: 'x',
        subscription_usage: {
          plan_type: 'plus',
          limit_reached: false,
          primary_window: { used_percent: 12.5, limit_window_seconds: 18000 },
          secondary_window: {
            used_percent: 40,
            reset_at: 1700400000,
            limit_window_seconds: 604800,
          },
          updated_at: 1700000000,
        },
      })
    )
    assert.deepEqual(snapshot, {
      plan_type: 'plus',
      limit_reached: false,
      primary_window: { used_percent: 12.5, limit_window_seconds: 18000 },
      secondary_window: {
        used_percent: 40,
        reset_at: 1700400000,
        limit_window_seconds: 604800,
      },
      updated_at: 1700000000,
    })
  })
})

describe('resolveRateLimitWindows from snapshot', () => {
  test('classifies windows by duration regardless of primary/secondary order', () => {
    const { fiveHourWindow, weeklyWindow } = resolveRateLimitWindows({
      plan_type: 'plus',
      rate_limit: {
        primary_window: { used_percent: 40, limit_window_seconds: 604800 },
        secondary_window: { used_percent: 12, limit_window_seconds: 18000 },
      },
    })
    assert.equal(weeklyWindow?.used_percent, 40)
    assert.equal(fiveHourWindow?.used_percent, 12)
  })

  test('free plan exposes a single weekly window', () => {
    const { fiveHourWindow, weeklyWindow } = resolveRateLimitWindows({
      plan_type: 'free',
      rate_limit: { primary_window: { used_percent: 100 } },
    })
    assert.equal(fiveHourWindow, null)
    assert.equal(weeklyWindow?.used_percent, 100)
  })
})

describe('windowLabel', () => {
  test('clamps percent and maps thresholds to variants', () => {
    assert.deepEqual(windowLabel({ used_percent: 12.4 }), {
      percent: 12.4,
      variant: 'info',
    })
    assert.deepEqual(windowLabel({ used_percent: 80 }), {
      percent: 80,
      variant: 'warning',
    })
    assert.deepEqual(windowLabel({ used_percent: 150 }), {
      percent: 100,
      variant: 'danger',
    })
    assert.deepEqual(windowLabel(null), { percent: 0, variant: 'info' })
  })
})

describe('resetCountdownSeconds', () => {
  test('derives whole seconds remaining from reset_at minus now', () => {
    assert.equal(
      resetCountdownSeconds({ reset_at: 1700003600 }, 1700000000000),
      3600
    )
    assert.equal(
      resetCountdownSeconds({ reset_at: 1700000090 }, 1700000000000),
      90
    )
    assert.equal(
      resetCountdownSeconds({ reset_at: 1700000045 }, 1700000000000),
      45
    )
  })

  test('floors sub-second remainders instead of rounding', () => {
    assert.equal(
      resetCountdownSeconds({ reset_at: 1700003600 }, 1700000000500),
      3599
    )
  })

  test('ignores reset_after_seconds and other relative fields', () => {
    const windowWithRelativeField = {
      reset_at: 1700003600,
      reset_after_seconds: 99,
    }
    assert.equal(
      resetCountdownSeconds(windowWithRelativeField, 1700000000000),
      3600
    )
  })

  test('returns null once the reset moment has passed', () => {
    assert.equal(
      resetCountdownSeconds({ reset_at: 1699999000 }, 1700000000000),
      null
    )
  })

  test('returns null when reset_at is missing, non-numeric, or non-positive', () => {
    assert.equal(
      resetCountdownSeconds(
        { used_percent: 0, limit_window_seconds: 18000 },
        1700000000000
      ),
      null
    )
    assert.equal(
      resetCountdownSeconds(
        { reset_at: 'soon' } as unknown as CodexRateLimitWindow,
        1700000000000
      ),
      null
    )
    assert.equal(resetCountdownSeconds({ reset_at: 0 }, 1700000000000), null)
    assert.equal(resetCountdownSeconds({ reset_at: -5 }, 1700000000000), null)
  })

  test('returns null for a null or undefined window', () => {
    assert.equal(resetCountdownSeconds(null, 1700000000000), null)
    assert.equal(resetCountdownSeconds(undefined, 1700000000000), null)
  })
})
