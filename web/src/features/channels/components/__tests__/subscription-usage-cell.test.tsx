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
const { SubscriptionSaturationBadge, SubscriptionUsageCell } =
  await import('../subscription-usage-cell')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type SubscriptionUsage = {
  bottleneck_percent: number
  refreshed_at: number
  saturated: boolean
}

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

describe('SubscriptionUsageCell', () => {
  after(() => {
    domWindow.close()
  })

  test('renders the bottleneck percent for fresh usage data', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 62.4,
      refreshed_at: Date.now() - 1000,
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={usage} />
    )

    assert.equal(container.textContent?.includes('62.4%'), true)
    assert.equal(container.textContent?.includes('No data yet'), false)

    await unmount()
  })

  test('renders "No data yet" when usage is undefined', async () => {
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={undefined} />
    )

    assert.equal(container.textContent?.includes('No data yet'), true)

    await unmount()
  })

  test('renders Stale badge and last known percent when refreshed_at is 11 minutes old', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 40,
      refreshed_at: Date.now() - 11 * 60_000,
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={usage} />
    )

    assert.equal(container.textContent?.includes('Stale'), true)
    assert.equal(container.textContent?.includes('40%'), true)

    await unmount()
  })
})

describe('SubscriptionSaturationBadge', () => {
  test('renders Saturated when usage.saturated is true', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 96,
      refreshed_at: Date.now(),
      saturated: true,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionSaturationBadge usage={usage} />
    )

    assert.equal(container.textContent?.includes('Saturated'), true)

    await unmount()
  })

  test('renders nothing when usage.saturated is false', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 40,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionSaturationBadge usage={usage} />
    )

    assert.equal(container.textContent?.includes('Saturated'), false)

    await unmount()
  })

  test('renders nothing when usage is undefined', async () => {
    const { container, unmount } = await renderInto(
      <SubscriptionSaturationBadge usage={undefined} />
    )

    assert.equal(container.textContent?.includes('Saturated'), false)

    await unmount()
  })

  test('trusts saturated=false even when bottleneck_percent exceeds 95', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 96,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionSaturationBadge usage={usage} />
    )

    assert.equal(container.textContent?.includes('Saturated'), false)

    await unmount()
  })
})
