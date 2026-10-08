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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { formatTimestampToDate } from '@/lib/format'

import type { Channel } from '../../types'
import { SubscriptionUsageBar } from '../subscription-usage-bar'

function channelWithOtherInfo(otherInfo: string): Channel {
  return { id: 1, type: 1, other_info: otherInfo } as unknown as Channel
}

function channelWithUsage(subscriptionUsage: object): Channel {
  return channelWithOtherInfo(
    JSON.stringify({ subscription_usage: subscriptionUsage })
  )
}

function renderBar(
  channel: Channel,
  props: { onOpen?: () => void; isLoading?: boolean } = {}
) {
  return render(
    <TooltipProvider>
      <SubscriptionUsageBar
        channel={channel}
        onOpen={props.onOpen ?? (() => {})}
        isLoading={props.isLoading ?? false}
      />
    </TooltipProvider>
  )
}

const bothWindows = {
  plan_type: 'team',
  primary_window: { used_percent: 29, limit_window_seconds: 18000 },
  secondary_window: { used_percent: 31, limit_window_seconds: 604800 },
}

describe('SubscriptionUsageBar', () => {
  test('renders tightly stacked 5h/7d rows with labels, percentages and per-window aria-labels when both windows exist', () => {
    renderBar(
      channelWithUsage({
        plan_type: 'team',
        primary_window: { used_percent: 29, limit_window_seconds: 18000 },
        secondary_window: { used_percent: 31, limit_window_seconds: 604800 },
      })
    )

    const fiveHLabel = screen.getByText('5h')
    const sevenDLabel = screen.getByText('7d')
    expect(
      fiveHLabel.compareDocumentPosition(sevenDLabel) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
    expect(screen.getByText('29%')).toBeInTheDocument()
    expect(screen.getByText('31%')).toBeInTheDocument()

    const progressBars = screen.getAllByRole('progressbar')
    expect(progressBars).toHaveLength(2)
    expect(progressBars[0]).toHaveAttribute('aria-label', '5-Hour Window: 29%')
    expect(progressBars[1]).toHaveAttribute('aria-label', 'Weekly Window: 31%')
    expect(progressBars[0].parentElement?.parentElement?.className).toContain(
      'gap-y-0.5'
    )
  })

  test.each([
    ['5h', 1700003600, 1700600000],
    ['7d', 1700600000, 1700003600],
  ])(
    'shows only the hovered %s row reset time, without an updated-at line',
    async (label, ownResetAt, otherResetAt) => {
      const user = userEvent.setup()
      const fiveHourResetAt = label === '5h' ? ownResetAt : otherResetAt
      const weeklyResetAt = label === '7d' ? ownResetAt : otherResetAt
      renderBar(
        channelWithUsage({
          plan_type: 'team',
          primary_window: {
            used_percent: 29,
            limit_window_seconds: 18000,
            reset_at: fiveHourResetAt,
          },
          secondary_window: {
            used_percent: 31,
            limit_window_seconds: 604800,
            reset_at: weeklyResetAt,
          },
          updated_at: 1700000000,
        })
      )

      await user.hover(screen.getByText(label))

      expect(
        await screen.findByText(
          `Resets at ${formatTimestampToDate(ownResetAt)}`
        )
      ).toBeInTheDocument()
      expect(
        screen.queryByText(`Resets at ${formatTimestampToDate(otherResetAt)}`)
      ).not.toBeInTheDocument()
      expect(screen.queryByText(/Updated at/)).not.toBeInTheDocument()
    }
  )

  test('tells the reset timer has not started when the 5h window has no reset_at', async () => {
    const user = userEvent.setup()
    renderBar(
      channelWithUsage({
        plan_type: 'team',
        primary_window: { used_percent: 0, limit_window_seconds: 18000 },
        secondary_window: {
          used_percent: 64,
          limit_window_seconds: 604800,
          reset_at: 1700600000,
        },
      })
    )

    await user.hover(screen.getByText('5h'))

    expect(
      await screen.findByText('Reset timer not started')
    ).toBeInTheDocument()
  })

  test('places a fixed-width, left-aligned percent after the progress bar', () => {
    renderBar(
      channelWithUsage({
        plan_type: 'team',
        primary_window: { used_percent: 2, limit_window_seconds: 18000 },
        secondary_window: { used_percent: 1, limit_window_seconds: 604800 },
      })
    )

    const progressBars = screen.getAllByRole('progressbar')
    const rows: Array<[string, string, HTMLElement]> = [
      ['5h', '2%', progressBars[0]],
      ['7d', '1%', progressBars[1]],
    ]
    for (const [label, percentText, bar] of rows) {
      const labelEl = screen.getByText(label)
      const percentEl = screen.getByText(percentText)
      expect(
        labelEl.compareDocumentPosition(bar) & Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy()
      expect(
        bar.compareDocumentPosition(percentEl) &
          Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy()
      expect(percentEl.className).not.toContain('text-right')
      expect(percentEl.className).toContain('w-9')
      expect(percentEl.className).toContain('tabular-nums')
      expect(bar.className).toContain('w-20')
      expect(bar.className).toContain('[&_[data-slot=progress-track]]:h-1.5')
    }
  })

  test('colors each row independently: >=95% destructive, <80% muted', () => {
    renderBar(
      channelWithUsage({
        plan_type: 'team',
        primary_window: { used_percent: 96, limit_window_seconds: 18000 },
        secondary_window: { used_percent: 40, limit_window_seconds: 604800 },
      })
    )

    const ninetySix = screen.getByText('96%')
    const forty = screen.getByText('40%')
    expect(ninetySix.className).toContain('text-destructive')
    expect(forty.className).toContain('text-muted-foreground')

    const progressBars = screen.getAllByRole('progressbar')
    expect(progressBars[0].className).toContain('bg-destructive')
    expect(progressBars[1].className).toContain('bg-info')
  })

  test.each([
    [79, 'text-muted-foreground', 'bg-info'],
    [80, 'text-warning', 'bg-warning'],
    [94, 'text-warning', 'bg-warning'],
    [95, 'text-destructive', 'bg-destructive'],
  ])(
    'grades a single-window row at %s%% with %s / %s',
    (usedPercent, textClass, barClass) => {
      renderBar(
        channelWithUsage({
          plan_type: 'free',
          primary_window: {
            used_percent: usedPercent,
            limit_window_seconds: 604800,
          },
        })
      )

      const percentText = screen.getByText(`${usedPercent}%`)
      expect(percentText.className).toContain(textClass)
      expect(screen.getByRole('progressbar').className).toContain(barClass)
    }
  )

  test('renders a single 7d-labeled row when only the weekly window exists', () => {
    renderBar(
      channelWithUsage({
        plan_type: 'free',
        primary_window: { used_percent: 31, limit_window_seconds: 604800 },
      })
    )

    expect(screen.getByText('7d')).toBeInTheDocument()
    expect(screen.getByText('31%')).toBeInTheDocument()
    expect(screen.queryByText('5h')).not.toBeInTheDocument()
    const progressBars = screen.getAllByRole('progressbar')
    expect(progressBars).toHaveLength(1)
    expect(progressBars[0]).toHaveAttribute('aria-label', 'Weekly Window: 31%')
  })

  test('renders a single 5h-labeled row when only the 5-hour window exists', () => {
    renderBar(
      channelWithUsage({
        plan_type: 'team',
        primary_window: { used_percent: 50, limit_window_seconds: 18000 },
      })
    )

    expect(screen.getByText('5h')).toBeInTheDocument()
    expect(screen.getByText('50%')).toBeInTheDocument()
    expect(screen.queryByText('7d')).not.toBeInTheDocument()
    const progressBars = screen.getAllByRole('progressbar')
    expect(progressBars).toHaveLength(1)
    expect(progressBars[0]).toHaveAttribute('aria-label', '5-Hour Window: 50%')
  })

  test('shows "-" with a "No usage data, click to query" tooltip when other_info is empty', async () => {
    const user = userEvent.setup()
    renderBar(channelWithOtherInfo(''))

    expect(screen.getByText('-')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    await user.hover(screen.getByText('-'))
    expect(
      await screen.findByText('No usage data, click to query')
    ).toBeInTheDocument()
  })

  test('makes the whole snapshot area one button that opens on click, Enter and Space', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    renderBar(channelWithUsage(bothWindows), { onOpen })

    const button = screen.getByRole('button')
    expect(button).toHaveAttribute('tabindex', '0')
    expect(button).toContainElement(screen.getByText('5h'))
    expect(button).toContainElement(screen.getByText('7d'))

    await user.tab()
    expect(button).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onOpen).toHaveBeenCalledTimes(1)
    await user.keyboard(' ')
    expect(onOpen).toHaveBeenCalledTimes(2)

    await user.click(button)
    expect(onOpen).toHaveBeenCalledTimes(3)
  })

  test('marks the entry aria-busy and ignores clicks while loading', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    renderBar(channelWithUsage(bothWindows), { onOpen, isLoading: true })

    const button = screen.getByRole('button')
    expect(button).toHaveAttribute('aria-busy', 'true')
    await user.click(button)
    expect(onOpen).not.toHaveBeenCalled()
  })

  test('makes the "-" placeholder a button that opens on click', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    renderBar(channelWithOtherInfo(''), { onOpen })

    const button = screen.getByRole('button')
    expect(button).toHaveTextContent('-')
    await user.click(button)
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  test('shows "-" when the snapshot has neither window', () => {
    renderBar(channelWithUsage({ plan_type: 'team', updated_at: 1700000000 }))

    expect(screen.getByText('-')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  test('shows "-" when other_info is not valid JSON', () => {
    renderBar(channelWithOtherInfo('{not valid json'))

    expect(screen.getByText('-')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })
})
