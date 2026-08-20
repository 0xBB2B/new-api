import { createInstance } from 'i18next'
import React, { act } from 'react'
import { createRoot } from 'react-dom/client'
import { I18nextProvider, initReactI18next } from 'react-i18next'
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

import { TooltipProvider } from '@/components/ui/tooltip'
import { formatTimestampToDate } from '@/lib/format'

import { formatRelativeTime } from '../../lib'
import {
  ClaudeUsageDialog,
  resolveClaudeUsageWindows,
} from '../dialogs/claude-usage-dialog'

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  nsSeparator: false,
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function renderInto(node: React.ReactElement) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <TooltipProvider>{node}</TooltipProvider>
      </I18nextProvider>
    )
  )
  return {
    container,
    async unmount() {
      await act(async () => root.unmount())
      container.remove()
    },
  }
}

function valuesForLabel(label: string): string[] {
  return [...document.querySelectorAll('div,span')]
    .filter((el) => el.textContent?.trim() === label)
    .map((el) => el.parentElement?.textContent?.replace(label, '').trim() ?? '')
}

describe('resolveClaudeUsageWindows', () => {
  test('reads top-level five_hour/seven_day-prefixed keys as separate windows', () => {
    const windows = resolveClaudeUsageWindows({
      five_hour: { utilization: 96.2 },
      seven_day: { utilization: 88 },
      seven_day_opus: { utilization: 41 },
    })

    expect(windows.length).toBe(3)
    const percents = windows.map((w) => w.percent).sort((a, b) => a - b)
    expect(percents).toEqual([41, 88, 96.2])
  })

  test('reads limits array entries by kind', () => {
    const windows = resolveClaudeUsageWindows({
      limits: [
        { kind: 'session', percent: 40 },
        { kind: 'weekly_all', percent: 85 },
      ],
    })

    expect(windows.length).toBe(2)
    const percents = windows.map((w) => w.percent).sort((a, b) => a - b)
    expect(percents).toEqual([40, 85])
  })

  test('excludes non-window objects such as extra_usage', () => {
    const windows = resolveClaudeUsageWindows({
      seven_day: { utilization: 12 },
      extra_usage: { is_enabled: true, utilization: 99 },
    })

    expect(windows.length).toBe(1)
    expect(windows[0].percent).toBe(12)
  })

  test('returns an empty array for non-object input', () => {
    expect(resolveClaudeUsageWindows('not an object')).toEqual([])
    expect(resolveClaudeUsageWindows(null)).toEqual([])
  })

  test('extracts resets_at and ignores other reset field names', () => {
    const windows = resolveClaudeUsageWindows({
      five_hour: { utilization: 10, resets_at: 1700000000 },
      seven_day: { utilization: 20, reset_at: 123 },
    })

    expect(windows.length).toBe(2)
    const byKey = new Map(windows.map((w) => [w.key, w]))
    expect(byKey.get('five_hour')?.resetsAt).toBe(1700000000)
    expect(byKey.get('seven_day')?.resetsAt).toBe(undefined)
  })
})

describe('ClaudeUsageDialog', () => {
  test('renders each window percent from the top-level object response', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          data: {
            five_hour: { utilization: 96.2 },
            seven_day: { utilization: 88 },
            seven_day_opus: { utilization: 41 },
          },
        }}
      />
    )

    const bodyText = document.body.textContent ?? ''
    expect(bodyText.includes('96.2%')).toBe(true)
    expect(bodyText.includes('88%')).toBe(true)
    expect(bodyText.includes('41%')).toBe(true)

    await unmount()
  })

  test('colors window percents by threshold level', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          data: {
            five_hour: { utilization: 96.2 },
            seven_day: { utilization: 88 },
            seven_day_opus: { utilization: 41 },
          },
        }}
      />
    )

    const percentEl = (text: string) =>
      [...document.querySelectorAll('div,span')].find(
        (el) => el.textContent?.trim() === text
      )
    expect(percentEl('96.2%')?.className.includes('text-rose-500')).toBe(true)
    expect(percentEl('88%')?.className.includes('text-amber-500')).toBe(true)
    const defaultEl = percentEl('41%')
    expect(defaultEl?.className.includes('text-rose-500')).toBe(false)
    expect(defaultEl?.className.includes('text-amber-500')).toBe(false)

    await unmount()
  })

  test('renders a formatted reset time when resets_at is present and a dash placeholder when it is missing', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          data: {
            five_hour: { utilization: 10, resets_at: 1700000000 },
            seven_day: { utilization: 20 },
          },
        }}
      />
    )

    const bodyText = document.body.textContent ?? ''
    expect(bodyText.includes('1700000000')).toBe(false)
    expect((bodyText.match(/Reset at:/g) ?? []).length).toBe(2)

    const values = valuesForLabel('Reset at:')
    expect(values.length).toBe(2)
    expect(values.includes(formatTimestampToDate(1700000000))).toBe(true)
    expect(values.includes('-')).toBe(true)

    await unmount()
  })

  test('shows a dash placeholder for the email and user ID fields when absent', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          data: { five_hour: { utilization: 10 } },
        }}
      />
    )

    const bodyText = document.body.textContent ?? ''
    expect(bodyText.includes('Email')).toBe(true)
    expect(bodyText.includes('User ID')).toBe(true)
    expect(valuesForLabel('Email').includes('-')).toBe(true)
    expect(valuesForLabel('User ID').includes('-')).toBe(true)

    await unmount()
  })

  test('renders the email and user ID field values when present in the response data', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          data: { email: 'a@b.c', user_id: 'u_123' },
        }}
      />
    )

    const bodyText = document.body.textContent ?? ''
    expect(bodyText.includes('a@b.c')).toBe(true)
    expect(bodyText.includes('u_123')).toBe(true)
    expect(valuesForLabel('Email').includes('a@b.c')).toBe(true)
    expect(valuesForLabel('User ID').includes('u_123')).toBe(true)

    await unmount()
  })

  test('renders the fixed window placeholder rows for a window without resets_at', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          data: { five_hour: { utilization: 10 } },
        }}
      />
    )

    const bodyText = document.body.textContent ?? ''
    expect(/Window:/.test(bodyText)).toBe(true)
    expect(/Used/.test(bodyText)).toBe(true)
    expect(/Reset at:/.test(bodyText)).toBe(true)
    expect(/Resets in:/.test(bodyText)).toBe(true)

    const windowRow = [...document.querySelectorAll('div')].find((el) =>
      el.textContent?.trim().startsWith('Window:')
    )
    expect(windowRow?.textContent?.trim()).toBe('Window: -')
    expect(valuesForLabel('Resets in:').includes('-')).toBe(true)

    await unmount()
  })

  test('normalizes empty email and user ID strings to a dash placeholder', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          data: { email: '', user_id: '  ' },
        }}
      />
    )

    expect(valuesForLabel('Email').includes('-')).toBe(true)
    expect(valuesForLabel('User ID').includes('-')).toBe(true)

    await unmount()
  })

  test('renders a relative reset time in the Resets in column when resets_at is present', async () => {
    const resetsAt = Math.floor(Date.now() / 1000) + 30 * 24 * 3600
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          data: { five_hour: { utilization: 10, resets_at: resetsAt } },
        }}
      />
    )

    const values = valuesForLabel('Resets in:')
    expect(values.includes(formatRelativeTime(resetsAt, 'en'))).toBe(true)

    await unmount()
  })

  test('masks the channel field when display overrides are provided', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        channelName='Claude 订阅 C'
        channelId={3}
        channelDisplayName='••••'
        channelDisplayId='••••'
        response={{
          success: true,
          upstream_status: 200,
          data: { five_hour: { utilization: 10 } },
        }}
      />
    )

    const bodyText = document.body.textContent ?? ''
    expect(bodyText.includes('•••• (#••••)')).toBe(true)
    expect(bodyText.includes('Claude 订阅 C')).toBe(false)

    await unmount()
  })

  test('renders the subscription_type value as a badge when present', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          subscription_type: 'max',
          data: {},
        }}
      />
    )

    expect(document.body.textContent?.includes('max')).toBe(true)

    await unmount()
  })

  test('does not render a subscription_type badge when the field is absent', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: true,
          upstream_status: 200,
          data: {},
        }}
      />
    )

    expect(document.body.textContent?.includes('max')).toBe(false)

    await unmount()
  })

  test('shows the upstream message in a top banner when success is false', async () => {
    const { unmount } = await renderInto(
      <ClaudeUsageDialog
        open
        onOpenChange={() => {}}
        response={{
          success: false,
          message: 'upstream status: 401',
        }}
      />
    )

    expect(document.body.textContent?.includes('upstream status: 401')).toBe(
      true
    )

    await unmount()
  })
})
