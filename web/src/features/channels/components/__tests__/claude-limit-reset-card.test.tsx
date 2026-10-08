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
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import type { ClaudeUsageResponse } from '../../api'
import { ClaudeLimitResetCard } from '../dialogs/claude-limit-reset-card'

const grantId = 'opus55-launch-promax-20260921'

function usageResponse(
  cooldownUntil: string | null = null
): ClaudeUsageResponse {
  return {
    success: true,
    upstream_status: 200,
    data: {
      cedar_ember: {
        eligible: true,
        at_limit: false,
        next_grant_id: grantId,
        cooldown_until: cooldownUntil,
        grants: [
          {
            id: grantId,
            label: 'Launch reset',
            resets_total: 1,
            resets_left: 1,
            ends_at: '2099-10-22T16:00:00+00:00',
            clears: ['five_hour', 'seven_day', 'seven_day_overage_included'],
            paused: false,
            usable_now: true,
            use_requires_limit: false,
          },
        ],
      },
      juniper_tide: { eligible: false, ineligible_reason: 'not_at_wall' },
    },
  } as ClaudeUsageResponse
}

function renderCard(
  response: ClaudeUsageResponse,
  options: { onRefresh?: () => void; isRefreshing?: boolean } = {}
) {
  render(
    <ClaudeLimitResetCard
      channelId={42}
      response={response}
      isRefreshing={options.isRefreshing ?? false}
      onRefresh={options.onRefresh ?? (() => {})}
    />
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('ClaudeLimitResetCard', () => {
  test('asks for confirmation, sends the full reset request and refreshes usage', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { success: true, message: 'ok' } })
    const onRefresh = vi.fn()
    const user = userEvent.setup()
    renderCard(usageResponse(), { onRefresh })

    expect(
      screen.getByText('Shows up when the 5-hour limit is used up')
    ).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Reset for free' }))
    let dialog = await screen.findByRole('alertdialog')
    expect(
      within(dialog).getByText(
        'This clears the windows listed below right away. Once used, this reset is gone.'
      )
    ).toBeInTheDocument()
    expect(dialog).toHaveTextContent('Clears')
    expect(dialog).toHaveTextContent(
      '5-Hour Window, Weekly Window, Fable weekly window'
    )
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(post).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Reset for free' }))
    dialog = await screen.findByRole('alertdialog')
    await user.click(
      within(dialog).getByRole('button', { name: 'Reset for free' })
    )

    expect(await screen.findByText('Reset completed')).toBeInTheDocument()
    expect(post).toHaveBeenCalledTimes(1)
    expect(post.mock.calls[0][0]).toBe('/api/channel/42/claude/usage/reset')
    expect(post.mock.calls[0][1]).toEqual({
      program: 'cedar_ember',
      grant_id: grantId,
      resets_left: 1,
    })
    expect(onRefresh).toHaveBeenCalledTimes(1)
    expect(screen.queryByText(/has been refreshed/)).not.toBeInTheDocument()
  })

  test('shows the server message and does not refresh when the reset fails', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: false, message: 'already used' },
    })
    const onRefresh = vi.fn()
    const user = userEvent.setup()
    renderCard(usageResponse(), { onRefresh })

    await user.click(screen.getByRole('button', { name: 'Reset for free' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(
      within(dialog).getByRole('button', { name: 'Reset for free' })
    )

    expect(await screen.findByText('Reset failed')).toBeInTheDocument()
    expect(screen.getByText('already used')).toBeInTheDocument()
    expect(onRefresh).not.toHaveBeenCalled()
  })

  test('disables the reset button and explains the cooldown while cooling down', () => {
    renderCard(usageResponse('2099-01-01T00:00:00Z'))

    expect(
      screen.getByRole('button', { name: 'Reset for free' })
    ).toBeDisabled()
    const terms = screen.getAllByRole('term').map((el) => el.textContent)
    const definitions = screen.getAllByRole('definition')
    expect(terms).toContain('Cooldown')
    expect(definitions[terms.indexOf('Cooldown')].textContent).toMatch(
      /^Available after 2099-01-01/
    )
    expect(screen.queryByText(/Cooling down/)).not.toBeInTheDocument()
  })

  test('shows the grant as title, upstream label and a field list', () => {
    renderCard(usageResponse())

    expect(screen.getByText('Full reset')).toBeInTheDocument()
    expect(screen.getByText('Launch reset')).toBeInTheDocument()

    const terms = screen.getAllByRole('term').map((el) => el.textContent)
    const definitions = screen
      .getAllByRole('definition')
      .map((el) => el.textContent)
    expect(terms).toEqual(['Resets left', 'Expires at', 'Clears'])
    expect(definitions[0]).toBe('1/1')
    expect(definitions[1]).toMatch(/^2099-10-2[23] \d{2}:\d{2}$/)
    expect(definitions[2]).toBe(
      '5-Hour Window, Weekly Window, Fable weekly window'
    )

    expect(screen.queryByText(/Clears: /)).not.toBeInTheDocument()
  })

  test('shows skeletons instead of reset buttons while usage is refreshing', () => {
    renderCard(usageResponse(), { isRefreshing: true })

    expect(
      screen.queryByRole('button', { name: 'Reset for free' })
    ).not.toBeInTheDocument()
  })

  test('joins cleared windows with the Chinese list separator in zhCN', async () => {
    await i18next.changeLanguage('zhCN')
    try {
      renderCard(usageResponse())
      const definitions = screen.getAllByRole('definition')
      expect(definitions.at(-1)).toHaveTextContent(
        '5-Hour Window、Weekly Window、Fable weekly window'
      )
    } finally {
      await i18next.changeLanguage('en')
    }
  })
})
