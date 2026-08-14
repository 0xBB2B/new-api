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
import { describe, test } from 'node:test'

import {
  estimateNextSubscriptionRefresh,
  isSubscriptionUsageExpired,
} from '../channel-utils'

describe('isSubscriptionUsageExpired', () => {
  const now = 1_700_000_000_000

  const cases: Array<{ name: string; refreshedAt: number; expected: boolean }> =
    [
      {
        name: '9 minutes ago is not expired',
        refreshedAt: now - 9 * 60_000,
        expected: false,
      },
      {
        name: 'exactly 10 minutes ago is not expired',
        refreshedAt: now - 10 * 60_000,
        expected: false,
      },
      {
        name: '11 minutes ago is expired',
        refreshedAt: now - 11 * 60_000,
        expected: true,
      },
    ]

  for (const { name, refreshedAt, expected } of cases) {
    test(name, () => {
      assert.equal(isSubscriptionUsageExpired(refreshedAt, now), expected)
    })
  }
})

describe('estimateNextSubscriptionRefresh', () => {
  const refreshedAt = 1_700_000_000_000

  test('type 57 (Codex) adds a 60 second period', () => {
    assert.equal(
      estimateNextSubscriptionRefresh(refreshedAt, 57),
      refreshedAt + 60_000
    )
  })

  test('type 61 (Claude Subscription) adds a 180 second period', () => {
    assert.equal(
      estimateNextSubscriptionRefresh(refreshedAt, 61),
      refreshedAt + 180_000
    )
  })
})
