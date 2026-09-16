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
import { render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { RateLimitWindowGrid } from '../dialogs/rate-limit-window-grid'

const NOW_MS = 1700000000000
const NOW_SEC = NOW_MS / 1000

function getCardByTitle(title: string): HTMLElement {
  const card = screen.getByText(title).closest('[data-slot="card"]')
  if (!card) {
    throw new Error(`card not found for title: ${title}`)
  }
  return card as HTMLElement
}

function getValueByLabel(card: HTMLElement, label: string): string | null {
  const labelNode = within(card).getByText(label)
  return labelNode.nextElementSibling?.textContent ?? null
}

describe('RateLimitWindowGrid resets-in countdown', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(NOW_MS)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  test('weekly window shows the countdown derived from reset_at, and Reset at stays populated', () => {
    render(
      <RateLimitWindowGrid
        weeklyWindow={{
          used_percent: 0,
          reset_at: NOW_SEC + 3600,
          limit_window_seconds: 604800,
        }}
      />
    )

    const card = getCardByTitle('Weekly Window')
    expect(getValueByLabel(card, 'Resets in:')).toBe('1h 0m')
    expect(getValueByLabel(card, 'Reset at:')).not.toBe('-')
  })

  test('five-hour window without reset_at shows "-" for both Reset at and Resets in', () => {
    render(
      <RateLimitWindowGrid
        fiveHourWindow={{ used_percent: 0, limit_window_seconds: 18000 }}
      />
    )

    const card = getCardByTitle('5-Hour Window')
    expect(getValueByLabel(card, 'Reset at:')).toBe('-')
    expect(getValueByLabel(card, 'Resets in:')).toBe('-')
  })

  test('ignores reset_after_seconds and still counts down from reset_at', () => {
    const fiveHourWindow = {
      used_percent: 0,
      reset_at: NOW_SEC + 3600,
      reset_after_seconds: 99,
      limit_window_seconds: 18000,
    }
    render(<RateLimitWindowGrid fiveHourWindow={fiveHourWindow} />)

    const card = getCardByTitle('5-Hour Window')
    expect(getValueByLabel(card, 'Resets in:')).toBe('1h 0m')
  })

  test('shows "-" once reset_at has already passed', () => {
    render(
      <RateLimitWindowGrid
        weeklyWindow={{
          used_percent: 0,
          reset_at: NOW_SEC - 60,
          limit_window_seconds: 604800,
        }}
      />
    )

    const card = getCardByTitle('Weekly Window')
    expect(getValueByLabel(card, 'Resets in:')).toBe('-')
  })

  test('formats each card independently: Mm Ss for the 5h window, Nh Mm for the weekly window', () => {
    render(
      <RateLimitWindowGrid
        fiveHourWindow={{
          used_percent: 12,
          reset_at: NOW_SEC + 90,
          limit_window_seconds: 18000,
        }}
        weeklyWindow={{
          used_percent: 40,
          reset_at: NOW_SEC + 3600,
          limit_window_seconds: 604800,
        }}
      />
    )

    const fiveHourCard = getCardByTitle('5-Hour Window')
    const weeklyCard = getCardByTitle('Weekly Window')
    expect(getValueByLabel(fiveHourCard, 'Resets in:')).toBe('1m 30s')
    expect(getValueByLabel(weeklyCard, 'Resets in:')).toBe('1h 0m')
  })
})
