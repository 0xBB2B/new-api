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

import { describe, expect, test } from 'vitest'

import {
  formatDurationSeconds,
  parseIsoTimestamp,
  parseSubscriptionUsageSnapshot,
  resetCountdownSeconds,
  resolveClaudeLimitResets,
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
    assert.equal(
      parseSubscriptionUsageSnapshot('{"subscription_usage":"nope"}'),
      null
    )
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

describe('formatDurationSeconds', () => {
  const identity = (key: string) => key

  test('shows Nd Hh Mm once the duration reaches a day, keeping zero middle segments', () => {
    assert.equal(formatDurationSeconds(360000, identity), '4d 4h 0m')
    assert.equal(formatDurationSeconds(90000, identity), '1d 1h 0m')
    assert.equal(formatDurationSeconds(86400, identity), '1d 0h 0m')
  })

  test('keeps the sub-day tiers unchanged', () => {
    assert.equal(formatDurationSeconds(86399, identity), '23h 59m')
    assert.equal(formatDurationSeconds(90, identity), '1m 30s')
    assert.equal(formatDurationSeconds(45, identity), '45s')
  })

  test('formats the weekly window duration for the Window: field', () => {
    assert.equal(formatDurationSeconds(604800, identity), '7d 0h 0m')
  })
})

const iso = (value: string) => Date.parse(value) / 1000
const nowSeconds = iso('2026-10-08T05:00:00Z')
const grantId = 'opus55-launch-promax-20260921'

const buildGrant = (overrides: Record<string, unknown> = {}) => ({
  id: grantId,
  label: 'Claude Opus 5.5 launch: one usage-limit reset for Pro and Max',
  resets_total: 1,
  resets_left: 1,
  starts_at: '2026-09-22T16:00:00+00:00',
  ends_at: '2026-10-22T16:00:00+00:00',
  clears: ['five_hour', 'seven_day', 'seven_day_overage_included'],
  paused: false,
  usable_now: true,
  use_requires_limit: false,
  ...overrides,
})

const buildCedarEmber = (
  cedar: Record<string, unknown> = {},
  grant: Record<string, unknown> = {}
) => ({
  eligible: true,
  ineligible_reason: null,
  at_limit: false,
  next_grant_id: grantId,
  cooldown_until: null,
  grants: [buildGrant(grant)],
  ...cedar,
})

const buildJuniperTide = (overrides: Record<string, unknown> = {}) => ({
  eligible: false,
  ineligible_reason: 'not_at_wall',
  available: false,
  next_available_at: null,
  ...overrides,
})

const buildResponse = (
  cedar: unknown = buildCedarEmber(),
  juniper: unknown = buildJuniperTide()
) => ({
  success: true,
  data: { cedar_ember: cedar, juniper_tide: juniper },
})

const resolveReady = (response: ReturnType<typeof buildResponse>) => {
  const result = resolveClaudeLimitResets(response, nowSeconds)
  if (result.state !== 'ready') {
    throw new Error(`expected ready, got ${result.state}`)
  }
  return result
}

describe('parseIsoTimestamp', () => {
  test('converts an ISO string to unix seconds', () => {
    expect(parseIsoTimestamp('2026-10-22T16:00:00+00:00')).toBe(
      iso('2026-10-22T16:00:00Z')
    )
  })

  test.each(['bad', null, undefined, '', 123])(
    'returns 0 for invalid input %s',
    (value) => {
      expect(parseIsoTimestamp(value)).toBe(0)
    }
  )
})

describe('resolveClaudeLimitResets state priority', () => {
  test('returns ready with both rows for the observed sample', () => {
    const result = resolveReady(buildResponse())

    expect(result.full).toEqual({
      available: true,
      label: 'Claude Opus 5.5 launch: one usage-limit reset for Pro and Max',
      resetsLeft: 1,
      resetsTotal: 1,
      endsAt: iso('2026-10-22T16:00:00Z'),
      clears: ['five_hour', 'seven_day', 'seven_day_overage_included'],
      cooldownUntil: 0,
      coolingDown: false,
      canReset: true,
      request: { program: 'cedar_ember', grant_id: grantId, resets_left: 1 },
    })
    expect(result.fiveHour).toEqual({
      canReset: false,
      reasonKind: 'not_at_wall',
    })
  })

  test('returns not_returned when both programs are null or missing', () => {
    expect(
      resolveClaudeLimitResets(
        { data: { cedar_ember: null, juniper_tide: null } },
        nowSeconds
      )
    ).toEqual({ state: 'not_returned' })
    expect(resolveClaudeLimitResets({ data: {} }, nowSeconds)).toEqual({
      state: 'not_returned',
    })
  })

  test('client_version outranks not_returned', () => {
    expect(
      resolveClaudeLimitResets(
        {
          limit_reset_unavailable: 'client_version',
          data: { cedar_ember: null, juniper_tide: null },
        },
        nowSeconds
      )
    ).toEqual({ state: 'client_version' })
  })

  test('is ready when only one program is returned', () => {
    const result = resolveReady(buildResponse(null, buildJuniperTide()))
    expect(result.full).toEqual({ available: false })
  })
})

describe('resolveClaudeLimitResets full reset row', () => {
  test.each([
    ['cooldown not yet over', '2026-10-08T05:10:00Z', false],
    ['cooldown already over', '2026-10-08T04:50:00Z', true],
    ['cooldown exactly now', '2026-10-08T05:00:00Z', true],
  ])('%s', (_name, cooldown, canReset) => {
    const result = resolveReady(
      buildResponse(buildCedarEmber({ cooldown_until: cooldown }))
    )
    const full = result.full
    expect(full).toMatchObject({
      canReset,
      coolingDown: !canReset,
      cooldownUntil: iso(cooldown),
    })
  })

  test('treats an unparsable cooldown as still cooling down', () => {
    const result = resolveReady(
      buildResponse(buildCedarEmber({ cooldown_until: 'soon' }))
    )
    expect(result.full).toMatchObject({
      canReset: false,
      coolingDown: true,
      cooldownUntil: 0,
    })
  })

  test.each([
    ['eligible is false', { eligible: false }, {}, false],
    ['grant is not usable now', {}, { usable_now: false }, false],
    ['grant is paused', {}, { paused: true }, false],
    ['no resets left', {}, { resets_left: 0 }, false],
    [
      'grant requires limit but account is not at limit',
      { at_limit: false },
      { use_requires_limit: true },
      false,
    ],
    [
      'grant requires limit and account is at limit',
      { at_limit: true },
      { use_requires_limit: true },
      true,
    ],
  ])('canReset when %s', (_name, cedar, grant, canReset) => {
    const result = resolveReady(buildResponse(buildCedarEmber(cedar, grant)))
    expect(result.full).toMatchObject({ available: true, canReset })
  })

  test('is unavailable when next_grant_id matches no grant', () => {
    const result = resolveReady(
      buildResponse({
        eligible: false,
        ineligible_reason: 'surface',
        grants: [],
        next_grant_id: null,
      })
    )
    expect(result.full).toEqual({ available: false })
  })

  test('is unavailable when next_grant_id points at a missing grant', () => {
    const result = resolveReady(
      buildResponse(buildCedarEmber({ next_grant_id: 'other' }))
    )
    expect(result.full).toEqual({ available: false })
  })
})

describe('resolveClaudeLimitResets five hour row', () => {
  test('can reset when eligible and available', () => {
    const result = resolveReady(
      buildResponse(
        buildCedarEmber(),
        buildJuniperTide({
          eligible: true,
          available: true,
          ineligible_reason: null,
        })
      )
    )
    expect(result.fiveHour).toEqual({
      canReset: true,
      request: { program: 'juniper_tide' },
    })
  })

  test.each([
    ['surface', 'client_identity'],
    ['cli_version', 'client_identity'],
  ])('maps ineligible_reason %s to %s', (reason, reasonKind) => {
    const result = resolveReady(
      buildResponse(
        buildCedarEmber(),
        buildJuniperTide({ ineligible_reason: reason })
      )
    )
    expect(result.fiveHour).toEqual({ canReset: false, reasonKind })
  })

  test('keeps unknown reasons as raw', () => {
    const result = resolveReady(
      buildResponse(
        buildCedarEmber(),
        buildJuniperTide({ ineligible_reason: 'weekly_used' })
      )
    )
    expect(result.fiveHour).toEqual({
      canReset: false,
      reasonKind: 'raw',
      rawReason: 'weekly_used',
    })
  })

  test('falls back to next_available when there is no reason', () => {
    const result = resolveReady(
      buildResponse(
        buildCedarEmber(),
        buildJuniperTide({
          ineligible_reason: null,
          next_available_at: '2026-10-09T00:00:00Z',
        })
      )
    )
    expect(result.fiveHour).toEqual({
      canReset: false,
      reasonKind: 'next_available',
      nextAvailableAt: iso('2026-10-09T00:00:00Z'),
    })
  })

  test('is none when no reason and no next_available_at', () => {
    const result = resolveReady(
      buildResponse(
        buildCedarEmber(),
        buildJuniperTide({ ineligible_reason: null })
      )
    )
    expect(result.fiveHour).toEqual({ canReset: false, reasonKind: 'none' })
  })

  test('is none when juniper_tide is null', () => {
    const result = resolveReady(buildResponse(buildCedarEmber(), null))
    expect(result.fiveHour).toEqual({ canReset: false, reasonKind: 'none' })
  })
})
