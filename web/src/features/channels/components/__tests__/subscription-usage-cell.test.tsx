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

import {
  SubscriptionSaturationBadge,
  SubscriptionUsageCell,
} from '../subscription-usage-cell'

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

function findLeafByText(root: ParentNode, text: string): Element | null {
  for (const el of root.querySelectorAll('*')) {
    if (el.children.length === 0 && el.textContent === text) {
      return el
    }
  }
  return null
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
  test('renders the bottleneck percent for fresh usage data', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 62.4,
      refreshed_at: Date.now() - 1000,
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={usage} />
    )

    expect(container.textContent?.includes('62.4%')).toBe(true)
    expect(container.textContent?.includes('No data yet')).toBe(false)

    await unmount()
  })

  test('renders "No data yet" when usage is undefined', async () => {
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={undefined} />
    )

    expect(container.textContent?.includes('No data yet')).toBe(true)

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

    expect(container.textContent?.includes('Stale')).toBe(true)
    expect(container.textContent?.includes('40%')).toBe(true)

    await unmount()
  })

  test('renders the progress bar before the percent number', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 62.4,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={usage} />
    )

    const progressEl = container.querySelector('[data-slot^="progress"]')
    const percentEl = findLeafByText(container, '62.4%')
    expect(progressEl).toBeTruthy()
    expect(percentEl).toBeTruthy()
    const position = percentEl && progressEl?.compareDocumentPosition(percentEl)
    expect(Boolean((position ?? 0) & Node.DOCUMENT_POSITION_FOLLOWING)).toBe(
      true
    )

    await unmount()
  })

  test('colors the percent text rose when usage is at or above 95%', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 96.2,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={usage} />
    )

    const percentEl = findLeafByText(container, '96.2%')
    expect(percentEl).toBeTruthy()
    expect(percentEl?.className.includes('rose')).toBe(true)

    await unmount()
  })

  test('colors the percent text amber when usage is at or above 80%', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 88,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={usage} />
    )

    const percentEl = findLeafByText(container, '88%')
    expect(percentEl).toBeTruthy()
    expect(percentEl?.className.includes('amber')).toBe(true)

    await unmount()
  })

  test('does not color the percent text rose or amber below 80% usage', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 62.4,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={usage} />
    )

    const percentEl = findLeafByText(container, '62.4%')
    expect(percentEl).toBeTruthy()
    expect(percentEl?.className.includes('rose')).toBe(false)
    expect(percentEl?.className.includes('amber')).toBe(false)

    await unmount()
  })

  test('marks a type 61 (Claude Subscription) cell as clickable', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 40,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 61 }} usage={usage} />
    )

    expect(container.innerHTML.includes('cursor-pointer')).toBe(true)

    await unmount()
  })

  test('marks a type 57 (Codex) cell as clickable', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 40,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionUsageCell channel={{ id: 1, type: 57 }} usage={usage} />
    )

    expect(container.innerHTML.includes('cursor-pointer')).toBe(true)

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
      <SubscriptionSaturationBadge usage={usage} channelType={61} />
    )

    expect(container.textContent?.includes('Saturated')).toBe(true)

    await unmount()
  })

  test('renders nothing when usage.saturated is false', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 40,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionSaturationBadge usage={usage} channelType={61} />
    )

    expect(container.textContent?.includes('Saturated')).toBe(false)

    await unmount()
  })

  test('renders nothing when usage is undefined', async () => {
    const { container, unmount } = await renderInto(
      <SubscriptionSaturationBadge usage={undefined} channelType={61} />
    )

    expect(container.textContent?.includes('Saturated')).toBe(false)

    await unmount()
  })

  test('trusts saturated=false even when bottleneck_percent exceeds 95', async () => {
    const usage: SubscriptionUsage = {
      bottleneck_percent: 96,
      refreshed_at: Date.now(),
      saturated: false,
    }
    const { container, unmount } = await renderInto(
      <SubscriptionSaturationBadge usage={usage} channelType={61} />
    )

    expect(container.textContent?.includes('Saturated')).toBe(false)

    await unmount()
  })
})
