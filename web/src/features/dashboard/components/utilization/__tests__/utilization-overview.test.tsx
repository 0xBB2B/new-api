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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { UtilizationOverview } from '../utilization-overview'

type Utilization = {
  total_users: number
  deep_users: number
  frequent_users: number
  moderate_users: number
  inactive_users: number
}

const normalData: Utilization = {
  total_users: 36,
  deep_users: 13,
  frequent_users: 21,
  moderate_users: 13,
  inactive_users: 2,
}
const emptyData: Utilization = {
  total_users: 0,
  deep_users: 0,
  frequent_users: 0,
  moderate_users: 0,
  inactive_users: 0,
}

let client: QueryClient
let utilizationHandler: () => Promise<unknown>
let exportEnabled: boolean

function mockApi() {
  return vi.spyOn(api, 'get').mockImplementation(async (url) => {
    switch (url) {
      case '/api/data/utilization':
        return utilizationHandler()
      case '/api/status':
        return {
          data: { success: true, data: { enable_data_export: exportEnabled } },
        }
      default:
        throw new Error(`Unexpected request: ${url}`)
    }
  })
}

function respondWith(data: Utilization) {
  return async () => ({ data: { success: true, message: '', data } })
}

function renderOverview() {
  return render(
    <QueryClientProvider client={client}>
      <UtilizationOverview />
    </QueryClientProvider>
  )
}

const offNotice = 'Data dashboard is off, statistics are incomplete'

beforeEach(() => {
  window.localStorage.clear()
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  utilizationHandler = respondWith(normalData)
  exportEnabled = true
})

afterEach(() => {
  cleanup()
  client.clear()
  vi.restoreAllMocks()
})

describe('UtilizationOverview data dashboard notice', () => {
  it('shows the incomplete-statistics notice when the data dashboard is off', async () => {
    exportEnabled = false
    mockApi()

    renderOverview()

    expect(await screen.findByText(offNotice)).toBeInTheDocument()
  })

  it('hides the notice when the data dashboard is on', async () => {
    mockApi()

    renderOverview()

    expect(await screen.findByText('Enabled users')).toBeInTheDocument()
    expect(screen.queryByText(offNotice)).not.toBeInTheDocument()
  })
})

describe('UtilizationOverview loading and failure', () => {
  it('shows neither statistic numbers nor an error while the request is pending', async () => {
    utilizationHandler = () => new Promise(() => undefined)
    const spy = mockApi()

    renderOverview()

    await vi.waitFor(() =>
      expect(spy).toHaveBeenCalledWith('/api/data/utilization')
    )
    expect(screen.queryByText('Enabled users')).not.toBeInTheDocument()
    expect(screen.queryByText('36')).not.toBeInTheDocument()
    expect(
      screen.queryByText('Failed to load utilization data')
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Retry' })
    ).not.toBeInTheDocument()
  })

  it('shows the error and refetches until numbers appear when Retry is clicked', async () => {
    utilizationHandler = async () => {
      throw new Error('network down')
    }
    const spy = mockApi()
    const user = userEvent.setup()

    renderOverview()

    expect(
      await screen.findByText('Failed to load utilization data')
    ).toBeInTheDocument()

    utilizationHandler = respondWith(normalData)
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByText('36')).toBeInTheDocument()
    const utilizationCalls = spy.mock.calls.filter(
      ([url]) => url === '/api/data/utilization'
    )
    expect(utilizationCalls).toHaveLength(2)
  })
})

describe('UtilizationOverview statistics', () => {
  it('shows the empty-state bar when there are no enabled regular users', async () => {
    utilizationHandler = respondWith(emptyData)
    mockApi()

    renderOverview()

    expect(await screen.findByText('No regular users yet')).toBeInTheDocument()
  })

  it.each([
    { count: '36', title: 'Enabled users', percent: null },
    { count: '13', title: 'Deep usage', percent: '36% of users' },
    { count: '21', title: 'Frequent usage', percent: '58% of users' },
    { count: '2', title: 'Inactive', percent: '6% of users' },
  ])(
    'shows $count with title $title in its card for the normal data set',
    async ({ count, title, percent }) => {
      mockApi()

      renderOverview()

      const card = (await screen.findByText(count)).parentElement as HTMLElement
      expect(card).toHaveTextContent(title)
      if (percent) {
        expect(card).toHaveTextContent(percent)
      }
    }
  )

  it('orders the four cards as enabled, deep, frequent, inactive', async () => {
    mockApi()

    renderOverview()

    const numbers = ['36', '13', '21', '2']
    const nodes = await Promise.all(numbers.map((n) => screen.findByText(n)))
    for (let i = 1; i < nodes.length; i++) {
      expect(
        nodes[i - 1].compareDocumentPosition(nodes[i]) &
          Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy()
    }
  })
})
