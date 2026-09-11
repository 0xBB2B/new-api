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
  parseCodexUsageSnapshot,
  resolveRateLimitWindows,
  windowLabel,
} from '../codex-usage'

describe('parseCodexUsageSnapshot', () => {
  test('returns null when other_info is empty, malformed, or lacks codex_usage', () => {
    assert.equal(parseCodexUsageSnapshot(''), null)
    assert.equal(parseCodexUsageSnapshot(undefined), null)
    assert.equal(parseCodexUsageSnapshot('{oops'), null)
    assert.equal(parseCodexUsageSnapshot('{"status_reason":"x"}'), null)
    assert.equal(parseCodexUsageSnapshot('{"codex_usage":"nope"}'), null)
  })

  test('reads the snapshot next to other keys', () => {
    const snapshot = parseCodexUsageSnapshot(
      JSON.stringify({
        status_reason: 'x',
        codex_usage: {
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
