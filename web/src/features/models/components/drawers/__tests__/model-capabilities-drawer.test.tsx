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
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { ModelsProvider } from '../../models-provider'
import type { Model } from '../../../types'
import { ModelMutateDrawer } from '../model-mutate-drawer'

const model = {
  id: 7,
  model_name: 'example-model',
  description: 'Original',
  status: 1,
  sync_official: 1,
  name_rule: 0,
  vendor_id: 3,
  endpoints: '',
  capabilities: '',
  supported_endpoints: ['openai'],
  created_time: 1,
  updated_time: 1,
} as Model

afterEach(() => {
  cleanup()
  i18next.addResource('en', 'translation', '、', '、')
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

function setup(current: Model = model) {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === `/api/models/${current.id}`) {
      return { data: { success: true, data: current } }
    }
    if (url === '/api/vendors/') {
      return {
        data: { success: true, data: { items: [{ id: 3, name: 'Vendor' }] } },
      }
    }
    return { data: { success: true, data: { items: [] } } }
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true, data: current } })
  const close = vi.fn()
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ModelsProvider>
        <ModelMutateDrawer open onOpenChange={close} currentRow={current} />
      </ModelsProvider>
    </QueryClientProvider>
  )
  return { put, close, user: userEvent.setup() }
}

async function section() {
  const heading = await screen.findByRole('heading', {
    name: 'Model capabilities',
  })
  return within(heading.closest('section, div')?.parentElement ?? heading)
}

const save = () => screen.getByRole('button', { name: /Update Model|Save metadata/ })

describe('model capabilities section in the model drawer', () => {
  it('sits after Matching Rules and before Endpoints', async () => {
    setup()
    const capabilities = await screen.findByRole('heading', {
      name: 'Model capabilities',
    })
    const rules = screen.getByRole('heading', { name: 'Matching Rules' })
    const endpoints = screen.getByRole('heading', { name: 'Endpoints' })
    expect(
      rules.compareDocumentPosition(capabilities) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
    expect(
      capabilities.compareDocumentPosition(endpoints) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  it('shows every field with its placeholder', async () => {
    setup()
    const s = await section()
    for (const name of ['Text', 'Image', 'File', 'Audio']) {
      expect(s.getAllByRole('checkbox', { name }).length).toBeGreaterThan(0)
    }
    for (const name of ['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max']) {
      expect(s.getByRole('checkbox', { name })).toBeInTheDocument()
    }
    expect(s.getByPlaceholderText('e.g. 1050000')).toBeInTheDocument()
    expect(s.getByPlaceholderText('e.g. 128000')).toBeInTheDocument()
  })

  it('saves with empty capabilities when nothing is configured', async () => {
    const { put, user } = setup()
    await section()
    await user.click(save())
    await waitFor(() => expect(put).toHaveBeenCalled())
    expect(put.mock.calls[0][1]).toMatchObject({ capabilities: '' })
  })

  it('submits canonical capabilities JSON from the form', async () => {
    const { put, user } = setup()
    const s = await section()
    await user.type(s.getByPlaceholderText('e.g. 1050000'), '1050000')
    await user.type(s.getByPlaceholderText('e.g. 128000'), '128000')
    await user.click(s.getAllByRole('checkbox', { name: 'Text' })[0])
    await user.click(s.getByRole('checkbox', { name: 'high' }))
    await user.click(s.getByRole('checkbox', { name: 'low' }))
    await user.click(save())
    await waitFor(() => expect(put).toHaveBeenCalled())
    expect(put.mock.calls[0][1]).toMatchObject({
      capabilities:
        '{"context_length":1050000,"top_provider":{"max_completion_tokens":128000},"architecture":{"input_modalities":["text"]},"reasoning":{"supported_efforts":["low","high"]}}',
    })
  })

  it('disables effort checkboxes under "no reasoning" and restores them afterwards', async () => {
    const { user } = setup()
    const s = await section()
    await user.click(s.getByRole('checkbox', { name: 'medium' }))
    await user.click(s.getByRole('switch'))
    expect(s.getByRole('checkbox', { name: 'medium' })).toBeDisabled()
    await user.click(s.getByRole('switch'))
    expect(s.getByRole('checkbox', { name: 'medium' })).toBeEnabled()
    expect(s.getByRole('checkbox', { name: 'medium' })).toBeChecked()
  })

  it('submits supported_efforts [] when "no reasoning" is on', async () => {
    const { put, user } = setup()
    const s = await section()
    await user.click(s.getByRole('switch'))
    await user.click(save())
    await waitFor(() => expect(put).toHaveBeenCalled())
    expect(put.mock.calls[0][1]).toMatchObject({
      capabilities: '{"reasoning":{"supported_efforts":[]}}',
    })
  })

  it.each(['0', '-5', '1.5', '9007199254740992'])(
    'blocks saving for invalid number %s',
    async (value) => {
      const { put, user } = setup()
      const s = await section()
      const input = s.getByPlaceholderText('e.g. 1050000')
      await user.type(input, value)
      await user.click(save())
      expect(put).not.toHaveBeenCalled()
      expect(input).toHaveAttribute('aria-invalid', 'true')
      expect(
        s.getByText('Enter an integer greater than 0')
      ).toBeInTheDocument()
    }
  )

  it('loads existing capabilities into the form', async () => {
    setup({
      ...model,
      capabilities:
        '{"context_length":1050000,"reasoning":{"supported_efforts":["max"]}}',
    })
    const s = await section()
    await waitFor(() =>
      expect(s.getByPlaceholderText('e.g. 1050000')).toHaveValue(1050000)
    )
    expect(s.getByRole('checkbox', { name: 'max' })).toBeChecked()
  })

  it('switches to JSON mode, shows {} when unconfigured, and back', async () => {
    const { user } = setup()
    const s = await section()
    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    expect(s.getByText('{}')).toBeInTheDocument()
    await user.click(s.getByRole('button', { name: /Visual mode/ }))
    expect(s.getByPlaceholderText('e.g. 1050000')).toBeInTheDocument()
  })

  it('shows the section description', async () => {
    setup()
    const s = await section()
    expect(
      s.getByText(
        'Values set here are returned to callers through /v1/models. Unset items are not returned.'
      )
    ).toBeInTheDocument()
  })

  it('keeps copy and format buttons in the JSON toolbar', async () => {
    const { user } = setup()
    const s = await section()
    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    expect(s.getByRole('button', { name: /Copy/ })).toBeInTheDocument()
    expect(s.getByRole('button', { name: /Format JSON/ })).toBeInTheDocument()
  })

  it('blocks save and disables switching back on malformed JSON', async () => {
    const { put, user } = setup()
    const s = await section()
    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    const editor = s.getByRole('textbox')
    fireEvent.input(editor, { target: { value: '{"context_length":' } })
    await waitFor(() =>
      expect(s.getByRole('button', { name: /Visual mode/ })).toBeDisabled()
    )
    expect(
      s.getByText(
        'JSON format error. Fix it before saving or switching back to visual mode.'
      )
    ).toBeInTheDocument()
    expect(s.getAllByText('Invalid JSON')).toHaveLength(1)
    expect(s.queryByText(/contains an unknown value/)).not.toBeInTheDocument()
    await user.click(save())
    expect(put).not.toHaveBeenCalled()
  })

  it('blocks save and lists the unknown value for invalid JSON content', async () => {
    const { put, user } = setup()
    const s = await section()
    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    fireEvent.input(s.getByRole('textbox'), {
      target: {
        value: '{"architecture":{"input_modalities":["video"]}}',
      },
    })
    expect(
      await s.findByText(/architecture\.input_modalities.*video/)
    ).toBeInTheDocument()
    expect(
      s.queryByText(/JSON format error. Fix it before saving/)
    ).not.toBeInTheDocument()
    await user.click(save())
    expect(put).not.toHaveBeenCalled()
  })

  it('carries JSON edits over to the form view', async () => {
    const { user } = setup()
    const s = await section()
    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    fireEvent.input(s.getByRole('textbox'), {
      target: { value: '{"context_length":4096}' },
    })
    await user.click(s.getByRole('button', { name: /Visual mode/ }))
    expect(s.getByPlaceholderText('e.g. 1050000')).toHaveValue(4096)
  })

  it('shows backend validation errors in the alert above the footer', async () => {
    const { put, user } = setup()
    put.mockResolvedValueOnce({
      data: { success: false, message: 'Model name must be 1 to 128 characters' },
    })
    await section()
    await user.click(save())
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Model name must be 1 to 128 characters'
    )
  })

  it('places the no-thinking switch before the effort checkboxes', async () => {
    setup()
    const s = await section()
    const toggle = s.getByRole('switch', { name: 'No thinking support' })
    for (const name of ['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max']) {
      expect(
        toggle.compareDocumentPosition(s.getByRole('checkbox', { name })) &
          Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy()
    }
  })

  it('shows one description per field', async () => {
    setup()
    const s = await section()
    expect(
      s.getAllByText('Leave all unchecked to keep this unset.')
    ).toHaveLength(2)
    expect(
      s.getAllByText('In tokens. Leave empty to keep this unset.')
    ).toHaveLength(2)
  })

  it('closes the drawer and toasts after a successful save', async () => {
    const success = vi.spyOn(toast, 'success').mockReturnValue('')
    const { put, close, user } = setup()
    await section()
    await user.click(save())
    await waitFor(() => expect(put).toHaveBeenCalled())
    await waitFor(() => expect(close).toHaveBeenCalledWith(false))
    expect(success).toHaveBeenCalledWith('Model metadata saved')
  })

  it('keeps the drawer open and the entered values after a failed save', async () => {
    const { put, close, user } = setup()
    put.mockResolvedValueOnce({ data: { success: false, message: 'boom' } })
    const s = await section()
    await user.type(s.getByPlaceholderText('e.g. 1050000'), '4096')
    await user.click(save())
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(close).not.toHaveBeenCalled()
    expect(s.getByPlaceholderText('e.g. 1050000')).toHaveValue(4096)
  })

  it('shows interpolated parameters of a server capability error', async () => {
    i18next.addResource('en', 'translation', '、', ' / ')
    const { put, user } = setup()
    put.mockResolvedValueOnce({
      data: {
        success: false,
        message: 'fallback message',
        message_key:
          '{{path}} contains an unknown value {{value}}. Allowed values: {{allowed}}',
        message_params: {
          path: 'architecture.input_modalities',
          value: 'video',
          allowed: ['text', 'image', 'file', 'audio'],
        },
      },
    })
    await section()
    await user.click(save())
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'architecture.input_modalities contains an unknown value video. Allowed values: text / image / file / audio'
    )
    expect(screen.queryByText(/fallback message/)).not.toBeInTheDocument()
  })

  it('lays the seven thinking levels out in a four-column grid', async () => {
    setup()
    const s = await section()
    const grid = s.getByRole('checkbox', { name: 'none' }).closest('div.grid')
    expect(grid).not.toBeNull()
    expect(grid?.className).toMatch(/(^|\s)(\w+:)?grid-cols-4(\s|$)/)
    expect(within(grid as HTMLElement).getAllByRole('checkbox')).toHaveLength(7)
  })

  it('puts an icon in each mode switch button', async () => {
    const { user } = setup()
    const s = await section()
    const toJson = s.getByRole('button', { name: /JSON mode/ })
    expect(toJson.querySelector('svg')).not.toBeNull()
    await user.click(toJson)
    const toVisual = s.getByRole('button', { name: /Visual mode/ })
    expect(toVisual.querySelector('svg')).not.toBeNull()
  })

  it('opens JSON mode with indented multi-line JSON and arrays on one line', async () => {
    const { user } = setup({
      ...model,
      capabilities:
        '{"context_length":1050000,"architecture":{"input_modalities":["text","image"]}}',
    })
    const s = await section()
    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    const value = (s.getByRole('textbox') as HTMLTextAreaElement).value
    expect(value.split('\n').length).toBeGreaterThan(1)
    expect(value).toMatch(/\n {2}"context_length": 1050000/)
    expect(value).toMatch(/\n {4}"input_modalities": \["text", ?"image"\]/)
  })

  it('keeps Visual mode clickable when the JSON is valid but a value is not recognised', async () => {
    const { user } = setup()
    const s = await section()
    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    fireEvent.input(s.getByRole('textbox'), {
      target: { value: '{"architecture":{"input_modalities":["video"]}}' },
    })
    await s.findByText(/architecture\.input_modalities.*video/)
    expect(s.getByRole('button', { name: /Visual mode/ })).toBeEnabled()
  })

  it('joins the allowed values in the validation hint with the localised list separator', async () => {
    i18next.addResource('en', 'translation', '、', ' / ')
    const { user } = setup()
    const s = await section()
    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    fireEvent.input(s.getByRole('textbox'), {
      target: {
        value:
          '{"architecture":{"input_modalities":["video"]},"reasoning":{"supported_efforts":["ultra"]}}',
      },
    })
    expect(
      await s.findByText(
        'architecture.input_modalities contains an unknown value video. Allowed values: text / image / file / audio'
      )
    ).toBeInTheDocument()
    expect(
      s.getByText(
        'reasoning.supported_efforts contains an unknown value ultra. Allowed values: none / minimal / low / medium / high / xhigh / max'
      )
    ).toBeInTheDocument()
  })

})

const MIXED_JSON =
  '{"context_length":-1,"top_provider":{"max_completion_tokens":128000},"architecture":{"input_modalities":["text","imgae","file"],"output_modalities":["text"]},"reasoning":{"supported_efforts":["low","ultra","medium"]},"foo":"bar"}'

async function openDiscardDialog(json = MIXED_JSON) {
  const { user } = setup()
  const s = await section()
  await user.click(s.getByRole('button', { name: /JSON mode/ }))
  fireEvent.input(s.getByRole('textbox'), { target: { value: json } })
  await s.findAllByText(/unknown value|not a capability field|must be/)
  await user.click(s.getByRole('button', { name: /Visual mode/ }))
  const dialog = await screen.findByRole('alertdialog')
  return { user, s, dialog }
}

describe('switching to visual mode with values that cannot be shown', () => {
  it('asks first and lists the values to discard in the order of the red hints', async () => {
    const { dialog } = await openDiscardDialog()
    const d = within(dialog)
    expect(d.getByText('Switch to visual mode?')).toBeInTheDocument()
    expect(
      d.getByText(
        'These values cannot be shown in visual mode and will be discarded after switching.'
      )
    ).toBeInTheDocument()
    const items = d.getAllByRole('listitem')
    expect(items.map((li) => li.textContent)).toEqual([
      'context_length: -1',
      'imgae in architecture.input_modalities',
      'ultra in reasoning.supported_efforts',
      'foo: "bar"',
    ])
    expect(d.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
    expect(
      d.getByRole('button', { name: 'Discard and switch' })
    ).toBeInTheDocument()
  })

  it('writes only the path and the value in monospace code elements', async () => {
    const { dialog } = await openDiscardDialog()
    const codes = within(dialog)
      .getAllByRole('listitem')
      .map((li) => [...li.querySelectorAll('code')].map((c) => c.textContent))
    expect(codes).toEqual([
      ['context_length', '-1'],
      ['imgae', 'architecture.input_modalities'],
      ['ultra', 'reasoning.supported_efforts'],
      ['foo', '"bar"'],
    ])
  })

  it('lists only the non-string item of a mixed array and keeps the string after discarding', async () => {
    const { user, s, dialog } = await openDiscardDialog(
      '{"architecture":{"input_modalities":[1,"text"]}}'
    )
    expect(
      within(dialog)
        .getAllByRole('listitem')
        .map((li) => li.textContent)
    ).toEqual(['1 in architecture.input_modalities'])
    await user.click(
      within(dialog).getByRole('button', { name: 'Discard and switch' })
    )
    const input = within(
      (await s.findByText('Input modalities')).parentElement as HTMLElement
    )
    expect(input.getByRole('checkbox', { name: 'Text' })).toBeChecked()
  })

  it('lists a repeated unknown value once without duplicate-key warnings', async () => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {})
    const { dialog } = await openDiscardDialog(
      '{"architecture":{"input_modalities":["video","video"]}}'
    )
    expect(
      within(dialog)
        .getAllByRole('listitem')
        .map((li) => li.textContent)
    ).toEqual(['video in architecture.input_modalities'])
    const logged = errors.mock.calls.map((call) => call.join(' ')).join('\n')
    expect(logged).not.toMatch(/same key/)
  })

  it('stays in JSON mode with the content and hints unchanged after Cancel', async () => {
    const { user, s, dialog } = await openDiscardDialog()
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect((s.getByRole('textbox') as HTMLTextAreaElement).value).toBe(
      MIXED_JSON
    )
    expect(s.getByText(/foo is not a capability field/)).toBeInTheDocument()
    expect(s.getByRole('button', { name: /Visual mode/ })).toBeInTheDocument()
  })

  it('keeps only the recognised parts after Discard and switch', async () => {
    const { user, s, dialog } = await openDiscardDialog()
    await user.click(
      within(dialog).getByRole('button', { name: 'Discard and switch' })
    )
    expect(await s.findByPlaceholderText('e.g. 1050000')).toHaveValue(null)
    expect(s.getByPlaceholderText('e.g. 128000')).toHaveValue(128000)
    const input = within(
      s.getByText('Input modalities').parentElement as HTMLElement
    )
    expect(input.getByRole('checkbox', { name: 'Text' })).toBeChecked()
    expect(input.getByRole('checkbox', { name: 'File' })).toBeChecked()
    expect(input.getByRole('checkbox', { name: 'Image' })).not.toBeChecked()
    expect(s.getByRole('checkbox', { name: 'low' })).toBeChecked()
    expect(s.getByRole('checkbox', { name: 'medium' })).toBeChecked()
    expect(s.getByRole('checkbox', { name: 'high' })).not.toBeChecked()
    expect(s.queryByText(/foo is not a capability field/)).not.toBeInTheDocument()

    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    const value = (s.getByRole('textbox') as HTMLTextAreaElement).value
    for (const gone of ['context_length', 'foo', 'imgae', 'ultra']) {
      expect(value).not.toContain(gone)
    }
  })

  it('switches without asking when every value is recognised', async () => {
    const { user } = setup()
    const s = await section()
    await user.click(s.getByRole('button', { name: /JSON mode/ }))
    fireEvent.input(s.getByRole('textbox'), {
      target: { value: '{"context_length":4096}' },
    })
    await user.click(s.getByRole('button', { name: /Visual mode/ }))
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(s.getByPlaceholderText('e.g. 1050000')).toHaveValue(4096)
  })

  it.each(['en', 'zh', 'zh-TW', 'fr', 'ru', 'ja', 'vi'])(
    '%s defines the discard dialog keys',
    async (locale) => {
      const mod = await import(`../../../../../i18n/locales/${locale}.json`)
      const translation = (mod.default ?? mod).translation as Record<
        string,
        string
      >
      const keys = [
        'Switch to visual mode?',
        'These values cannot be shown in visual mode and will be discarded after switching.',
        '<path/>: <value/>',
        '<value/> in <path/>',
        'Discard and switch',
      ]
      expect(keys.filter((key) => !(key in translation))).toEqual([])
      expect('{{path}}: {{value}}' in translation).toBe(false)
      expect('{{value}} in {{path}}' in translation).toBe(false)
    }
  )

  it('zh wording of the discard dialog', async () => {
    const mod = await import('../../../../../i18n/locales/zh.json')
    const translation = (mod.default ?? mod).translation as Record<
      string,
      string
    >
    expect(translation['Switch to visual mode?']).toBe('切换到可视模式？')
    expect(
      translation[
        'These values cannot be shown in visual mode and will be discarded after switching.'
      ]
    ).toBe('下面这些值在可视模式里无法表示，切换后会丢弃。')
    expect(translation['<path/>: <value/>']).toBe('<path/>：<value/>')
    expect(translation['<value/> in <path/>']).toBe('<path/> 里的 <value/>')
    expect(translation['Discard and switch']).toBe('丢弃并切换')
  })
})
