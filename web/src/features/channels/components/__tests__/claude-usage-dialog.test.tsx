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
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const React = await import('react')
const { act } = React
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { TooltipProvider } = await import('@/components/ui/tooltip')
const { ClaudeUsageDialog, resolveClaudeUsageWindows } = await import(
  '../dialogs/claude-usage-dialog'
)

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
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

describe('resolveClaudeUsageWindows', () => {
  test('reads top-level five_hour/seven_day-prefixed keys as separate windows', () => {
    const windows = resolveClaudeUsageWindows({
      five_hour: { utilization: 96.2 },
      seven_day: { utilization: 88 },
      seven_day_opus: { utilization: 41 },
    })

    assert.equal(windows.length, 3)
    const percents = windows.map((w) => w.percent).sort((a, b) => a - b)
    assert.deepEqual(percents, [41, 88, 96.2])
  })

  test('reads limits array entries by kind', () => {
    const windows = resolveClaudeUsageWindows({
      limits: [
        { kind: 'session', percent: 40 },
        { kind: 'weekly_all', percent: 85 },
      ],
    })

    assert.equal(windows.length, 2)
    const percents = windows.map((w) => w.percent).sort((a, b) => a - b)
    assert.deepEqual(percents, [40, 85])
  })

  test('excludes non-window objects such as extra_usage', () => {
    const windows = resolveClaudeUsageWindows({
      seven_day: { utilization: 12 },
      extra_usage: { is_enabled: true, utilization: 99 },
    })

    assert.equal(windows.length, 1)
    assert.equal(windows[0].percent, 12)
  })

  test('returns an empty array for non-object input', () => {
    assert.deepEqual(resolveClaudeUsageWindows('not an object'), [])
    assert.deepEqual(resolveClaudeUsageWindows(null), [])
  })

  test('extracts resets_at and ignores other reset field names', () => {
    const windows = resolveClaudeUsageWindows({
      five_hour: { utilization: 10, resets_at: 1700000000 },
      seven_day: { utilization: 20, reset_at: 123 },
    })

    assert.equal(windows.length, 2)
    const byKey = new Map(windows.map((w) => [w.key, w]))
    assert.equal(byKey.get('five_hour')?.resetsAt, 1700000000)
    assert.equal(byKey.get('seven_day')?.resetsAt, undefined)
  })
})

describe('ClaudeUsageDialog', () => {
  after(() => {
    domWindow.close()
  })

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
    assert.equal(bodyText.includes('96.2%'), true)
    assert.equal(bodyText.includes('88%'), true)
    assert.equal(bodyText.includes('41%'), true)

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
    assert.equal(percentEl('96.2%')?.className.includes('text-rose-500'), true)
    assert.equal(percentEl('88%')?.className.includes('text-amber-500'), true)
    const defaultEl = percentEl('41%')
    assert.equal(defaultEl?.className.includes('text-rose-500'), false)
    assert.equal(defaultEl?.className.includes('text-amber-500'), false)

    await unmount()
  })

  test('renders a formatted reset time only when resets_at is present', async () => {
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
    assert.equal(bodyText.includes('Reset at:'), true)
    assert.equal(bodyText.includes('1700000000'), false)
    assert.equal((bodyText.match(/Reset at:/g) ?? []).length, 1)

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

    assert.equal(document.body.textContent?.includes('max'), true)

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

    assert.equal(document.body.textContent?.includes('max'), false)

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

    assert.equal(
      document.body.textContent?.includes('upstream status: 401'),
      true
    )

    await unmount()
  })
})
