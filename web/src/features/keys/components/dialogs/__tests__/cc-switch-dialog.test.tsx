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
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { CCSwitchDialog } from '../cc-switch-dialog'

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })
})

afterEach(() => {
  queryClient.clear()
})

function renderDialog(models = ['gpt-5.4', 'claude-sonnet-4-6']) {
  for (const endpoint of [undefined, 'gemini']) {
    queryClient.setQueryData(['user-models-ccswitch', endpoint], {
      success: true,
      data: models,
    })
  }
  render(
    <QueryClientProvider client={queryClient}>
      <CCSwitchDialog open onOpenChange={vi.fn()} tokenKey='test-only' />
    </QueryClientProvider>
  )
}

describe('CC Switch model selection', () => {
  it.each(['Claude', 'Codex', 'Gemini'])(
    'opens %s models outside the clipping dialog and keeps the dialog open after selection',
    async (app) => {
      renderDialog()
      const user = userEvent.setup()
      await user.click(screen.getByRole('radio', { name: app }))
      const input = screen.getByRole('combobox', { name: 'Primary Model' })

      await user.click(input)

      expect(input).toHaveAttribute('aria-expanded', 'true')
      const list = await screen.findByRole('listbox')
      const dialog = screen.getByRole('dialog', { name: 'Import to CC Switch' })
      // The dialog is translated and clips overflow. Its popup must escape
      // that containing block to remain aligned and fully visible.
      expect(dialog).not.toContainElement(list)
      await user.click(screen.getByRole('option', { name: 'gpt-5.4' }))
      await waitFor(() => expect(input).toHaveValue('gpt-5.4'))
      expect(input).toHaveAttribute('aria-expanded', 'false')
      expect(dialog).toBeVisible()
    }
  )

  it('filters model names and supports keyboard selection and Escape without closing the dialog', async () => {
    renderDialog()
    const user = userEvent.setup()
    await user.click(screen.getByRole('radio', { name: 'Codex' }))
    const input = screen.getByRole('combobox', { name: 'Primary Model' })
    await user.click(input)
    await user.type(input, 'sonnet')

    expect(
      screen.queryByRole('option', { name: 'gpt-5.4' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('option', { name: 'claude-sonnet-4-6' })
    ).toBeVisible()
    await user.keyboard('{ArrowDown}{Enter}')
    await waitFor(() => expect(input).toHaveValue('claude-sonnet-4-6'))
    await user.click(input)
    await user.keyboard('{Escape}')

    expect(input).toHaveValue('claude-sonnet-4-6')
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByRole('dialog')).toBeVisible()
  })

  it('shows an empty result when no models are available and updates an open dropdown when models arrive', async () => {
    renderDialog([])
    const user = userEvent.setup()
    await user.click(screen.getByRole('radio', { name: 'Codex' }))
    const input = screen.getByRole('combobox', { name: 'Primary Model' })
    await user.click(screen.getByRole('button', { name: 'Primary Model' }))

    expect(await screen.findByText('No models found')).toBeVisible()
    await act(async () => {
      queryClient.setQueryData(['user-models-ccswitch', undefined], {
        success: true,
        data: ['gpt-5.4'],
      })
    })
    await user.click(await screen.findByRole('option', { name: 'gpt-5.4' }))
    await waitFor(() => expect(input).toHaveValue('gpt-5.4'))
  })

  it('lets users edit the provider name without opening a model dropdown', async () => {
    renderDialog()
    const user = userEvent.setup()
    const input = screen.getByRole('textbox', { name: 'Name' })

    await user.clear(input)
    await user.type(input, 'Development')

    expect(input).toHaveValue('Development')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })
})

describe('CC Switch virtual models', () => {
  let get: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    get = vi.spyOn(api, 'get').mockImplementation(async (_url, config) => {
      const endpoint = (config?.params as { endpoint?: string } | undefined)
        ?.endpoint
      const byEndpoint: Record<string, string[]> = {
        anthropic: ['gpt-5.5', 'auto-gpt'],
        openai: ['gpt-5.5', 'auto-chat'],
        gemini: ['gpt-5.5'],
      }
      return {
        data: {
          success: true,
          data: byEndpoint[endpoint ?? ''] ?? [
            'gpt-5.5',
            'auto-gpt',
            'auto-chat',
          ],
        },
      }
    })
  })

  function expectNoEndpointParam() {
    expect(get).toHaveBeenCalled()
    for (const call of get.mock.calls) {
      const config = call[1] as { params?: { endpoint?: string } } | undefined
      expect(config?.params?.endpoint).toBeUndefined()
    }
  }

  afterEach(() => {
    get.mockRestore()
  })

  function renderFetchingDialog() {
    render(
      <QueryClientProvider client={queryClient}>
        <CCSwitchDialog open onOpenChange={vi.fn()} tokenKey='test-only' />
      </QueryClientProvider>
    )
  }

  it('lists every virtual model after the real ones for Codex', async () => {
    renderFetchingDialog()
    const user = userEvent.setup()
    await user.click(screen.getByRole('radio', { name: 'Codex' }))
    await user.click(screen.getByRole('combobox', { name: 'Primary Model' }))

    const options = (await screen.findAllByRole('option')).map(
      (option) => option.textContent
    )
    expect(options).toEqual(['gpt-5.5', 'auto-gpt', 'auto-chat'])
    expectNoEndpointParam()
  })

  it.each(['Primary Model', 'Haiku Model', 'Sonnet Model', 'Opus Model'])(
    'lists virtual models in the Claude %s dropdown',
    async (field) => {
      renderFetchingDialog()
      const user = userEvent.setup()
      await user.click(screen.getByRole('combobox', { name: field }))

      expect(
        await screen.findByRole('option', { name: 'auto-gpt' })
      ).toBeVisible()
      expect(screen.getByRole('option', { name: 'auto-chat' })).toBeVisible()
      expectNoEndpointParam()
    }
  )

  it('requests endpoint=gemini and lists no virtual model for Gemini', async () => {
    renderFetchingDialog()
    const user = userEvent.setup()
    await user.click(screen.getByRole('radio', { name: 'Gemini' }))
    await user.click(screen.getByRole('combobox', { name: 'Primary Model' }))

    expect(await screen.findByRole('option', { name: 'gpt-5.5' })).toBeVisible()
    expect(
      screen.queryByRole('option', { name: 'auto-gpt' })
    ).not.toBeInTheDocument()
    expect(get).toHaveBeenCalledWith(
      '/api/user/models',
      expect.objectContaining({
        params: expect.objectContaining({ endpoint: 'gemini' }),
      })
    )
  })
})
