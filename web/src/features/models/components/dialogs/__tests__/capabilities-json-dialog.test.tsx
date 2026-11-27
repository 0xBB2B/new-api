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
import { AxiosError } from 'axios'
import i18next from 'i18next'
import { useState } from 'react'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { useAuthStore } from '@/stores/auth-store'

import { ModelsDialogs } from '../../models-dialogs'
import { ModelsPrimaryButtons } from '../../models-primary-buttons'
import { ModelsProvider } from '../../models-provider'
import { CapabilitiesJsonDialog } from '../capabilities-json-dialog'

vi.mock('@/lib/copy-to-clipboard', () => ({
  copyToClipboard: vi.fn(async () => true),
}))

const capabilities = {
  'gpt-6.2-sol': { context_length: 1050000 },
  'qwen4-max': {
    reasoning: { supported_efforts: ['none', 'low', 'high', 'xhigh'] },
  },
}
const formatted = [
  '{',
  '  "gpt-6.2-sol": {',
  '    "context_length": 1050000',
  '  },',
  '  "qwen4-max": {',
  '    "reasoning": {',
  '      "supported_efforts": ["none", "low", "high", "xhigh"]',
  '    }',
  '  }',
  '}',
].join('\n')

const loaded = {
  capabilities,
  non_exact: [{ model_name: 'gpt-6', name_rule: 1 }],
  unconfigured: ['glm-5.2'],
}

const previewData = {
  version: 'v-preview-1',
  items: [
    {
      model_name: 'new-model',
      action: 'create',
      before: null,
      after: { context_length: 4096 },
    },
    {
      model_name: 'gpt-6.2-sol',
      action: 'update',
      before: {
        context_length: 128000,
        reasoning: { supported_efforts: ['low'] },
      },
      after: {
        context_length: 256000,
        reasoning: { supported_efforts: ['low'] },
      },
    },
    {
      model_name: 'old-model',
      action: 'clear',
      before: { context_length: 8192 },
      after: null,
    },
    {
      model_name: 'qwen4-max',
      action: 'unchanged',
      before: { reasoning: { supported_efforts: ['low'] } },
      after: { reasoning: { supported_efforts: ['low'] } },
    },
  ],
  errors: [],
  summary: { create: 1, update: 1, clear: 1, unchanged: 1 },
}

const previewWithErrors = {
  version: 'v-preview-2',
  items: [],
  errors: [
    {
      model_name: 'bad-model',
      message: {
        key: '{{path}} must be a positive integer',
        params: { path: 'context_length' },
      },
    },
  ],
  summary: { create: 0, update: 0, clear: 0, unchanged: 0 },
}

function httpError(status: number, message: string) {
  const error = new AxiosError(`Request failed with status code ${status}`)
  error.response = {
    data: { success: false, message },
    status,
    statusText: String(status),
    headers: {},
    config: { headers: {} },
  } as typeof error.response
  return error
}

function mountDialog(options: { data?: unknown; getError?: boolean } = {}) {
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/models/capabilities') {
      if (options.getError) throw httpError(500, 'boom')
      return { data: { success: true, data: options.data ?? loaded } }
    }
    return { data: { success: true, data: { items: [] } } }
  })
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true, data: previewData } })
  const put = vi.spyOn(api, 'put').mockResolvedValue({
    data: { success: true, data: { create: 1, update: 1, clear: 1 } },
  })
  const onOpenChange = vi.fn()
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const invalidate = vi.spyOn(client, 'invalidateQueries')
  render(
    <QueryClientProvider client={client}>
      <ModelsProvider>
        <CapabilitiesJsonDialog open onOpenChange={onOpenChange} />
      </ModelsProvider>
    </QueryClientProvider>
  )
  return { get, post, put, onOpenChange, invalidate, user: userEvent.setup() }
}

const editor = () => screen.findByRole('textbox')
const edit = async (value: string) =>
  fireEvent.input(await editor(), { target: { value } })
const previewButton = () =>
  screen.getByRole('button', { name: 'Preview changes' })

async function goToStep2(user: ReturnType<typeof userEvent.setup>) {
  await edit('{"new-model":{"context_length":4096}}')
  await user.click(previewButton())
  await screen.findByRole('button', { name: /Replace and save/ })
}

function footerOf() {
  const preview = screen.getByRole('button', { name: 'Preview changes' })
  let footer = screen.getByRole('button', { name: 'Download JSON file' })
  while (!footer.contains(preview) && footer.parentElement) {
    footer = footer.parentElement
  }
  return footer
}

function blockAncestors(name: string, allNames: string[]) {
  const matches = screen.getAllByText(name, { exact: true })
  let el = matches.at(-1) ?? matches[0]
  const path = [el]
  const others = allNames.filter((n) => n !== name)
  while (el.parentElement) {
    const text = el.parentElement.textContent ?? ''
    if (others.some((n) => text.includes(n))) break
    el = el.parentElement
    path.push(el)
  }
  return path
}

function blockOf(name: string, allNames: string[]) {
  return within(blockAncestors(name, allNames).at(-1) as HTMLElement)
}

const follows = (a: Element, b: Element) =>
  Boolean(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING)

const RULES_TEXT =
  "Saving replaces everything. After saving, each model's capabilities will be exactly what is written here. Fields that are omitted or set to null will be cleared. Removing a model from the JSON clears its capabilities but keeps the record. Models without a record in Model Management get a new exact-match record automatically."

function previewWith(
  items: Record<string, unknown>[],
  errors: Record<string, unknown>[] = []
) {
  const summary = { create: 0, update: 0, clear: 0, unchanged: 0 }
  for (const item of items) summary[item.action as keyof typeof summary]++
  return { data: { success: true, data: { version: 'v-x', items, errors, summary } } }
}

afterEach(() => {
  cleanup()
  i18next.addResource('en', 'translation', '、', '、')
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

describe('entry on the models page', () => {
  it('puts the Capabilities JSON button right after Sync pricing and opens the dialog', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role: 100 })
    vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/models/capabilities') {
        return { data: { success: true, data: loaded } }
      }
      return { data: { success: true, data: { items: [] } } }
    })
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const user = userEvent.setup()
    render(
      <QueryClientProvider client={client}>
        <ModelsProvider>
          <ModelsPrimaryButtons />
          <ModelsDialogs />
        </ModelsProvider>
      </QueryClientProvider>
    )
    const names = screen
      .getAllByRole('button')
      .map((b) => b.getAttribute('aria-label') ?? b.textContent?.trim())
    expect(names).toEqual([
      'Sync metadata',
      'Sync pricing',
      'Capabilities JSON',
      'Model square order',
      'Add Model',
      'Open menu',
    ])
    await user.click(screen.getByRole('button', { name: 'Capabilities JSON' }))
    expect(
      await screen.findByRole('dialog', { name: 'Capabilities JSON' })
    ).toBeInTheDocument()
  })
})

describe('step 1: view and edit', () => {
  it('shows the header, steps and the formatted capabilities from the server', async () => {
    const { get } = mountDialog()
    expect(await editor()).toHaveValue(formatted)
    expect(get).toHaveBeenCalledWith('/api/models/capabilities')
    expect(
      screen.getByRole('heading', { name: 'Capabilities JSON' })
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        'These are the current capabilities of all models, organized by model name, in the same format as /v1/models. You can copy or download them, edit them directly, or paste new content and save.'
      )
    ).toBeInTheDocument()
    expect(screen.getByText('1. Review and edit')).toHaveAttribute(
      'aria-current',
      'step'
    )
    expect(screen.getByText('2. Preview and save')).not.toHaveAttribute(
      'aria-current'
    )
    expect(
      screen.getByText(
        'Saving replaces everything. After saving, each model\'s capabilities will be exactly what is written here. Fields that are omitted or set to null will be cleared. Removing a model from the JSON clears its capabilities but keeps the record. Models without a record in Model Management get a new exact-match record automatically.'
      )
    ).toBeInTheDocument()
  })

  it('states non-exact and unconfigured records as two sentences instead of tags', async () => {
    i18next.addResource('en', 'translation', '、', ' / ')
    mountDialog({
      data: {
        ...loaded,
        non_exact: [
          { model_name: 'gpt-6', name_rule: 1 },
          { model_name: '-preview', name_rule: 3 },
        ],
      },
    })
    await editor()
    expect(
      screen.getByText('3 records are not listed below')
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        /2 records are not exact-match records: gpt-6 \(Prefix Match\) \/ -preview \(Suffix Match\)\./
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        /They are not specific model names\. Edit them in the model drawer; saving here will not touch them\./
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        /1 records have no capabilities configured yet: glm-5\.2\./
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText(/To configure it, add its name below\./)
    ).toBeInTheDocument()
    expect(screen.queryByText(/To configure them/)).not.toBeInTheDocument()
    for (const name of ['gpt-6', '-preview', 'glm-5.2']) {
      expect(screen.queryByText(name, { exact: true })).not.toBeInTheDocument()
    }
  })

  it('uses the plural add-names sentence when two records are unconfigured', async () => {
    i18next.addResource('en', 'translation', '、', ' / ')
    mountDialog({
      data: { ...loaded, non_exact: [], unconfigured: ['glm-5.2', 'glm-6'] },
    })
    await editor()
    expect(
      screen.getByText(
        /2 records have no capabilities configured yet: glm-5\.2 \/ glm-6\./
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText(/To configure them, add their names below\./)
    ).toBeInTheDocument()
    expect(screen.queryByText(/To configure it/)).not.toBeInTheDocument()
  })

  it('labels the editor above it with the field name', async () => {
    mountDialog()
    const textarea = await editor()
    const label = screen.getByText('Capability configuration (JSON)')
    expect(follows(label, textarea)).toBe(true)
  })

  it('shows the replace-everything note as plain text below the editor, outside a bordered alert', async () => {
    mountDialog()
    const textarea = await editor()
    const note = screen.getByText(RULES_TEXT)
    expect(note.closest('[data-slot="alert"]')).toBeNull()
    expect(note.closest('[role="alert"]')).toBeNull()
    expect(follows(textarea, note)).toBe(true)
  })

  it('shows the malformed-JSON message after the replace-everything note', async () => {
    mountDialog()
    await edit('{"gpt-6.2-sol":')
    const note = screen.getByText(RULES_TEXT)
    const error = screen.getByText(
      'The JSON is malformed. Fix it before previewing.'
    )
    expect(follows(note, error)).toBe(true)
  })

  it('uses one complete sample entry as the empty-editor placeholder', async () => {
    mountDialog({ data: { capabilities: {}, non_exact: [], unconfigured: [] } })
    const placeholder = (await editor()).getAttribute('placeholder') ?? ''
    const sample = JSON.parse(placeholder) as Record<string, unknown>
    const entries = Object.values(sample)
    expect(entries).toHaveLength(1)
    expect(Object.keys(sample)).toEqual(['Model name'])
    const entry = entries[0] as Record<string, Record<string, unknown>> & {
      context_length: unknown
    }
    expect(Object.keys(entry).sort()).toEqual([
      'architecture',
      'context_length',
      'reasoning',
      'top_provider',
    ])
    expect(Object.keys(entry.architecture).sort()).toEqual([
      'input_modalities',
      'output_modalities',
    ])
    expect(Object.keys(entry.top_provider)).toEqual(['max_completion_tokens'])
    expect(Object.keys(entry.reasoning)).toEqual(['supported_efforts'])
  })

  it('hides the step bar while loading and after a load error', async () => {
    vi.spyOn(api, 'get').mockReturnValue(new Promise(() => {}))
    const view = render(
      <QueryClientProvider client={new QueryClient()}>
        <ModelsProvider>
          <CapabilitiesJsonDialog open onOpenChange={() => {}} />
        </ModelsProvider>
      </QueryClientProvider>
    )
    await screen.findByText('Loading...')
    expect(screen.queryByText('1. Review and edit')).not.toBeInTheDocument()
    expect(screen.queryByText('2. Preview and save')).not.toBeInTheDocument()
    view.unmount()
    vi.restoreAllMocks()

    mountDialog({ getError: true })
    await screen.findByText('Oops! Something went wrong')
    expect(screen.queryByText('1. Review and edit')).not.toBeInTheDocument()
    expect(screen.queryByText('2. Preview and save')).not.toBeInTheDocument()
  })

  it('opens at 768px wide', async () => {
    mountDialog()
    await editor()
    const dialog = screen.getByRole('dialog')
    expect(dialog.className).toContain('sm:max-w-3xl')
    expect(dialog.className).not.toContain('sm:max-w-4xl')
  })

  it('hides the notice when every record is listed', async () => {
    mountDialog({ data: { capabilities, non_exact: [], unconfigured: [] } })
    await editor()
    expect(
      screen.queryByText(/records are not listed below/)
    ).not.toBeInTheDocument()
  })

  it('shows the model count and a disabled preview until something changes, then flags unsaved changes', async () => {
    mountDialog()
    await editor()
    expect(screen.getByText('2 models have capabilities configured')).toBeInTheDocument()
    expect(previewButton()).toBeDisabled()
    await edit('{"gpt-6.2-sol":{"context_length":2000000}}')
    expect(screen.getByText('You have unsaved changes')).toBeInTheDocument()
    expect(
      screen.queryByText('2 models have capabilities configured')
    ).not.toBeInTheDocument()
    expect(previewButton()).toBeEnabled()
  })

  it('shows the empty state with a sample placeholder and disabled download and preview', async () => {
    mountDialog({ data: { capabilities: {}, non_exact: [], unconfigured: [] } })
    const textarea = await editor()
    expect(textarea).toHaveValue('')
    expect(textarea).toHaveAttribute('placeholder')
    expect(textarea.getAttribute('placeholder')).toContain('context_length')
    expect(
      screen.getByText(
        'No model has capabilities configured yet. You can paste JSON directly.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Download JSON file' })
    ).toBeDisabled()
    expect(previewButton()).toBeDisabled()
  })

  it('keeps the status text and the Close, Download and Preview buttons in a footer apart from the editor', async () => {
    const { user, onOpenChange } = mountDialog()
    const textarea = await editor()
    const footer = footerOf()
    expect(footer).toHaveTextContent('2 models have capabilities configured')
    expect(footer.contains(textarea)).toBe(false)
    expect(footer).toContainElement(previewButton())
    await user.click(within(footer).getByRole('button', { name: 'Close' }))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('replaces the empty-data footer with the unsaved-changes status once valid JSON is entered', async () => {
    mountDialog({ data: { capabilities: {}, non_exact: [], unconfigured: [] } })
    await edit('{"new-model":{"context_length":4096}}')
    expect(screen.getByText('You have unsaved changes')).toBeInTheDocument()
    expect(
      screen.queryByText(
        'No model has capabilities configured yet. You can paste JSON directly.'
      )
    ).not.toBeInTheDocument()
    expect(previewButton()).toBeEnabled()
  })

  it('shows the empty footer and no add-names hint when only unconfigured records exist', async () => {
    mountDialog({
      data: { capabilities: {}, non_exact: [], unconfigured: ['glm-5.2'] },
    })
    await editor()
    expect(screen.queryByText(/To configure (them|it)/)).not.toBeInTheDocument()
    expect(
      screen.getByText(
        'No model has capabilities configured yet. You can paste JSON directly.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Download JSON file' })
    ).toBeDisabled()
    expect(previewButton()).toBeDisabled()
  })

  it('keeps Download and Preview present but disabled while loading and after a load error', async () => {
    vi.spyOn(api, 'get').mockReturnValue(new Promise(() => {}))
    const client = new QueryClient()
    const view = render(
      <QueryClientProvider client={client}>
        <ModelsProvider>
          <CapabilitiesJsonDialog open onOpenChange={() => {}} />
        </ModelsProvider>
      </QueryClientProvider>
    )
    await screen.findByText('Loading...')
    expect(
      screen.getByRole('button', { name: 'Download JSON file' })
    ).toBeDisabled()
    expect(previewButton()).toBeDisabled()
    view.unmount()
    vi.restoreAllMocks()

    mountDialog({ getError: true })
    await screen.findByText('Oops! Something went wrong')
    expect(
      screen.getByRole('button', { name: 'Download JSON file' })
    ).toBeDisabled()
    expect(previewButton()).toBeDisabled()
  })

  it('blocks preview for malformed JSON and explains why', async () => {
    mountDialog()
    await edit('{"gpt-6.2-sol":')
    expect(screen.getAllByText('Invalid JSON').length).toBeGreaterThan(0)
    expect(
      screen.getByText('The JSON is malformed. Fix it before previewing.')
    ).toBeInTheDocument()
    expect(previewButton()).toBeDisabled()
  })

  it('shows a loading state while the capabilities are being read', async () => {
    vi.spyOn(api, 'get').mockReturnValue(new Promise(() => {}))
    const client = new QueryClient()
    render(
      <QueryClientProvider client={client}>
        <ModelsProvider>
          <CapabilitiesJsonDialog open onOpenChange={() => {}} />
        </ModelsProvider>
      </QueryClientProvider>
    )
    expect(await screen.findByText('Loading...')).toBeInTheDocument()
  })

  it('shows a retryable error when reading fails', async () => {
    const { get, user } = mountDialog({ getError: true })
    expect(
      await screen.findByText('Oops! Something went wrong')
    ).toBeInTheDocument()
    expect(screen.getByText('Failed to load model capabilities')).toBeInTheDocument()
    const before = get.mock.calls.length
    await user.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(get.mock.calls.length).toBeGreaterThan(before))
  })

  it('copies the editor content from the toolbar', async () => {
    const { user } = mountDialog()
    await editor()
    const success = vi.spyOn(toast, 'success')
    await user.click(screen.getByRole('button', { name: 'Copy' }))
    await waitFor(() => expect(copyToClipboard).toHaveBeenCalledWith(formatted))
    expect(success).toHaveBeenCalledWith('Copied to clipboard')
  })

  it('downloads the current editor content, including unsaved edits, with a dated file name', async () => {
    const { user } = mountDialog()
    const edited = '{"gpt-6.2-sol":{"context_length":2000000}}'
    await edit(edited)
    let blob: Blob | undefined
    const createObjectURL = vi.fn((b: Blob) => {
      blob = b
      return 'blob:capabilities'
    })
    Object.assign(URL, { createObjectURL, revokeObjectURL: vi.fn() })
    const anchors: HTMLAnchorElement[] = []
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(
      function (this: HTMLAnchorElement) {
        anchors.push(this)
      }
    )
    await user.click(screen.getByRole('button', { name: 'Download JSON file' }))
    expect(anchors).toHaveLength(1)
    expect(anchors[0].download).toMatch(
      /^model-capabilities-\d{4}-\d{2}-\d{2}\.json$/
    )
    expect(await blob?.text()).toBe(edited)
  })

  it('closes without saving', async () => {
    const { user, onOpenChange, post, put } = mountDialog()
    await editor()
    await user.click(
      within(footerOf()).getByRole('button', { name: 'Close' })
    )
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(post).not.toHaveBeenCalled()
    expect(put).not.toHaveBeenCalled()
  })

  it('closes from the top-right button and discards unsaved edits', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: loaded },
    })
    const post = vi.spyOn(api, 'post')
    const put = vi.spyOn(api, 'put')
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    function Harness() {
      const [open, setOpen] = useState(true)
      return (
        <QueryClientProvider client={client}>
          <ModelsProvider>
            <button type='button' onClick={() => setOpen(true)}>
              Reopen
            </button>
            <CapabilitiesJsonDialog open={open} onOpenChange={setOpen} />
          </ModelsProvider>
        </QueryClientProvider>
      )
    }
    const user = userEvent.setup()
    render(<Harness />)
    await edit('{"x":{"context_length":1}}')
    const footer = footerOf()
    const topRight = screen
      .getAllByRole('button', { name: 'Close' })
      .find((b) => !footer.contains(b))
    expect(topRight).toBeDefined()
    await user.click(topRight as HTMLElement)
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(post).not.toHaveBeenCalled()
    expect(put).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Reopen' }))
    expect(await editor()).toHaveValue(formatted)
  })
})

describe('step 2: preview', () => {
  it('posts the edited object to preview and shows badges, clear warning, blocks and summary', async () => {
    const { user, post } = mountDialog()
    await goToStep2(user)
    expect(post).toHaveBeenCalledWith('/api/models/capabilities/preview', {
      capabilities: { 'new-model': { context_length: 4096 } },
    })
    expect(screen.getByText('2. Preview and save')).toHaveAttribute(
      'aria-current',
      'step'
    )
    for (const badge of [
      'Create 1',
      'Update 1',
      'Clear 1',
      'Unchanged 1',
    ]) {
      expect(screen.getByText(badge)).toBeInTheDocument()
    }
    expect(
      screen.getByText('Capabilities of 1 models will be cleared')
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        'old-model is not in the JSON. If you did not mean to remove it, go back and add it again.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText('Will create 1, update 1, and clear 1 records')
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Replace and save 3 changes' })
    ).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Back to editing' })).toBeEnabled()
  })

  it('renders each model block by action and lists only changed fields', async () => {
    const { user } = mountDialog()
    await goToStep2(user)
    const names = ['new-model', 'gpt-6.2-sol', 'old-model', 'qwen4-max']

    const created = blockOf('new-model', names)
    expect(created.getByText('—')).toBeInTheDocument()
    expect(
      created.getByText(
        'This model has no record in Model Management yet. An exact-match record will be created.'
      )
    ).toBeInTheDocument()

    const updated = blockOf('gpt-6.2-sol', names)
    expect(updated.getByText('Field')).toBeInTheDocument()
    expect(updated.getByText('Current value')).toBeInTheDocument()
    expect(updated.getByText('Change to')).toBeInTheDocument()
    expect(updated.getByText(/128,?000/)).toBeInTheDocument()
    expect(updated.getByText(/256,?000/)).toBeInTheDocument()
    expect(updated.queryByText(/reasoning|efforts/i)).not.toBeInTheDocument()

    const cleared = blockOf('old-model', names)
    expect(cleared.getByText('Cleared (not configured)')).toBeInTheDocument()
    expect(
      cleared.getByText(
        'The model is not in the JSON, so its capabilities will be cleared. The record itself is kept.'
      )
    ).toBeInTheDocument()

    const unchanged = blockOf('qwen4-max', names)
    expect(
      unchanged.getByText('Same as the current configuration. Nothing will change.')
    ).toBeInTheDocument()
    expect(unchanged.queryByText('Field')).not.toBeInTheDocument()
  })

  it('shows the clear badge in the destructive style', async () => {
    const { user } = mountDialog()
    await goToStep2(user)
    expect(screen.getByText('Clear 1').className).toMatch(/text-destructive/)
  })

  it('formats numbers with locale separators and spells out an emptied efforts list', async () => {
    const { user, post } = mountDialog()
    post.mockResolvedValue({
      data: {
        success: true,
        data: {
          ...previewData,
          items: [
            {
              model_name: 'big-model',
              action: 'update',
              before: {
                context_length: 1050000,
                reasoning: { supported_efforts: ['low', 'high'] },
              },
              after: {
                context_length: 2100000,
                reasoning: { supported_efforts: [] },
              },
            },
          ],
          summary: { create: 0, update: 1, clear: 0, unchanged: 0 },
        },
      },
    })
    await edit('{"big-model":{"context_length":2100000}}')
    await user.click(previewButton())
    await screen.findByRole('button', { name: /Replace and save/ })
    const block = blockOf('big-model', ['big-model'])
    expect(block.getByText('1,050,000')).toBeInTheDocument()
    expect(block.getByText('2,100,000')).toBeInTheDocument()
    expect(block.getByText('No thinking support')).toBeInTheDocument()
  })

  it('shows the interpolated validation sentence for structured server messages', async () => {
    const { user, post } = mountDialog()
    i18next.addResource('en', 'translation', '、', ' / ')
    post.mockResolvedValue({
      data: {
        success: true,
        data: {
          ...previewWithErrors,
          errors: [
            {
              model_name: 'bad-model',
              message: {
                key: '{{path}} contains an unknown value {{value}}. Allowed values: {{allowed}}',
                params: {
                  path: 'reasoning.supported_efforts',
                  value: 'ultra',
                  allowed: ['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'],
                },
              },
            },
          ],
        },
      },
    })
    await edit('{"bad-model":{"reasoning":{"supported_efforts":["ultra"]}}}')
    await user.click(previewButton())
    expect(
      await screen.findByText(
        'reasoning.supported_efforts contains an unknown value ultra. Allowed values: none / minimal / low / medium / high / xhigh / max'
      )
    ).toBeInTheDocument()
  })

  it('names the compared fields in readable words', async () => {
    const { user, post } = mountDialog()
    post.mockResolvedValue(
      previewWith([
        {
          model_name: 'full-model',
          action: 'create',
          before: null,
          after: {
            context_length: 2000000,
            top_provider: { max_completion_tokens: 256000 },
            architecture: {
              input_modalities: ['text', 'image'],
              output_modalities: ['text'],
            },
            reasoning: { supported_efforts: ['low', 'high'] },
          },
        },
      ])
    )
    await goToStep2(user)
    const block = blockOf('full-model', ['full-model'])
    for (const label of [
      'Context length',
      'Max output tokens',
      'Input modalities',
      'Output modalities',
      'Thinking levels',
    ]) {
      expect(block.getByText(label)).toBeInTheDocument()
    }
    for (const path of [
      'context_length',
      'top_provider.max_completion_tokens',
      'architecture.input_modalities',
      'architecture.output_modalities',
      'reasoning.supported_efforts',
    ]) {
      expect(block.queryByText(path)).not.toBeInTheDocument()
    }
  })

  it('lists every existing field of a cleared model as current value to cleared', async () => {
    const { user, post } = mountDialog()
    post.mockResolvedValue(
      previewWith([
        {
          model_name: 'old-model',
          action: 'clear',
          before: {
            context_length: 8192,
            architecture: { input_modalities: ['text'] },
            reasoning: { supported_efforts: [] },
          },
          after: null,
        },
        {
          model_name: 'same-model',
          action: 'unchanged',
          before: { context_length: 1 },
          after: { context_length: 1 },
        },
      ])
    )
    await goToStep2(user)
    const block = blockOf('old-model', ['old-model', 'same-model'])
    expect(block.getByText('Field')).toBeInTheDocument()
    expect(block.getByText('Current value')).toBeInTheDocument()
    expect(block.getByText('Change to')).toBeInTheDocument()
    expect(block.getByText('Context length')).toBeInTheDocument()
    expect(block.getByText('8,192')).toBeInTheDocument()
    expect(block.getByText('Input modalities')).toBeInTheDocument()
    expect(block.getByText('text')).toBeInTheDocument()
    expect(block.getByText('Thinking levels')).toBeInTheDocument()
    expect(block.getByText('No thinking support')).toBeInTheDocument()
    expect(block.getAllByText('Cleared (not configured)')).toHaveLength(3)
  })

  it('orders model blocks as create, update, clear, unchanged, by name within each kind', async () => {
    const { user, post } = mountDialog()
    const item = (model_name: string, action: string) => ({
      model_name,
      action,
      before: action === 'create' ? null : { context_length: 1 },
      after: action === 'clear' ? null : { context_length: 2 },
    })
    post.mockResolvedValue(
      previewWith([
        item('z-clear', 'clear'),
        item('b-same', 'unchanged'),
        item('y-update', 'update'),
        item('m-new', 'create'),
        item('a-update', 'update'),
        item('c-new', 'create'),
      ])
    )
    await goToStep2(user)
    const order = ['c-new', 'm-new', 'a-update', 'y-update', 'z-clear', 'b-same']
    const elements = order.map((name) =>
      screen.getByText(name, { exact: true })
    )
    for (let i = 1; i < elements.length; i++) {
      expect(follows(elements[i - 1], elements[i])).toBe(true)
    }
  })

  it('puts the create explanation below the comparison table', async () => {
    const { user } = mountDialog()
    await goToStep2(user)
    const created = blockOf('new-model', [
      'new-model',
      'gpt-6.2-sol',
      'old-model',
      'qwen4-max',
    ])
    const table = created.getByRole('table')
    const note = created.getByText(
      'This model has no record in Model Management yet. An exact-match record will be created.'
    )
    expect(follows(table, note)).toBe(true)
  })

  it('shows the clear warning in the plain alert style', async () => {
    const { user } = mountDialog()
    await goToStep2(user)
    const alert = screen
      .getByText('Capabilities of 1 models will be cleared')
      .closest('[data-slot="alert"]')
    expect(alert).not.toBeNull()
    expect(alert?.className).not.toMatch(/destructive/)
  })

  it('draws model blocks without a bordered card', async () => {
    const { user } = mountDialog()
    await goToStep2(user)
    const names = ['new-model', 'gpt-6.2-sol', 'old-model', 'qwen4-max']
    for (const name of names) {
      for (const el of blockAncestors(name, names)) {
        expect(el.getAttribute('data-slot')).not.toBe('card')
        expect(
          [...el.classList].filter((c) => /^(border|rounded-lg|rounded-xl)$/.test(c))
        ).toEqual([])
      }
    }
  })

  it('counts validation failures per model and groups a model\'s problems in one row', async () => {
    const { user, post } = mountDialog()
    post.mockResolvedValue(
      previewWith(
        [],
        [
          {
            model_name: 'bad-a',
            message: {
              key: '{{path}} must be an integer from 1 to 9007199254740991',
              params: { path: 'context_length' },
            },
          },
          {
            model_name: 'bad-b',
            message: {
              key: '{{path}} is not a capability field',
              params: { path: 'foo' },
            },
          },
          {
            model_name: 'bad-a',
            message: {
              key: '{{path}} is not a capability field',
              params: { path: 'bar' },
            },
          },
        ]
      )
    )
    await edit('{"bad-a":{},"bad-b":{}}')
    await user.click(previewButton())
    expect(await screen.findByText('Validation failed 2')).toBeInTheDocument()
    expect(
      screen.getByText('2 entries failed validation and cannot be saved now')
    ).toBeInTheDocument()
    expect(
      screen.getByText('Entries that failed validation (2)')
    ).toBeInTheDocument()
    const row = screen.getByText('bad-a').closest('tr') as HTMLElement
    expect(row).toHaveTextContent(
      'context_length must be an integer from 1 to 9007199254740991'
    )
    expect(row).toHaveTextContent('bar is not a capability field')
    const table = row.closest('table') as HTMLElement
    expect(within(table).getAllByRole('row')).toHaveLength(3)
  })

  it('lets long problem sentences wrap inside the cell', async () => {
    const { user, post } = mountDialog()
    post.mockResolvedValue({ data: { success: true, data: previewWithErrors } })
    await edit('{"bad-model":{"context_length":-1}}')
    await user.click(previewButton())
    const cell = (await screen.findByText(
      'context_length must be a positive integer'
    )).closest('td') as HTMLElement
    expect(cell.className).toMatch(/whitespace-normal|break-words/)
    expect(cell.className).not.toContain('whitespace-nowrap')
  })

  it('orders the failed entries by where the model first appears in the submitted JSON', async () => {
    const { user, post } = mountDialog()
    const error = (model_name: string) => ({
      model_name,
      message: {
        key: '{{path}} is not a capability field',
        params: { path: 'foo' },
      },
    })
    post.mockResolvedValue(
      previewWith([], [error('gpt-x'), error('qwen4-max')])
    )
    await edit('{"qwen4-max":{"foo":1},"gpt-x":{"foo":1}}')
    await user.click(previewButton())
    const table = (await screen.findByText('gpt-x')).closest(
      'table'
    ) as HTMLElement
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows.map((row) => row.querySelector('td')?.textContent)).toEqual([
      'qwen4-max',
      'gpt-x',
    ])
  })

  it('orders failed entries by model key even when another model name appears earlier as a value', async () => {
    const { user, post } = mountDialog()
    const error = (model_name: string) => ({
      model_name,
      message: {
        key: '{{path}} is not a capability field',
        params: { path: 'foo' },
      },
    })
    post.mockResolvedValue(previewWith([], [error('text'), error('bravo')]))
    await edit(
      '{"alpha":{"architecture":{"input_modalities":["text"]}},"bravo":{"foo":1},"text":{"foo":1}}'
    )
    await user.click(previewButton())
    const table = (await screen.findByText('bravo', { exact: true })).closest(
      'table'
    ) as HTMLElement
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows.map((row) => row.querySelector('td')?.textContent)).toEqual([
      'bravo',
      'text',
    ])
  })

  it('shows the failed-entry count at the left of the footer', async () => {
    const { user, post } = mountDialog()
    post.mockResolvedValue({ data: { success: true, data: previewWithErrors } })
    await edit('{"bad-model":{"context_length":-1}}')
    await user.click(previewButton())
    const back = await screen.findByRole('button', { name: 'Back to editing' })
    const footer = back.parentElement as HTMLElement
    expect(footer.firstElementChild).toHaveTextContent('1 entries failed validation')
    expect(footer).not.toHaveTextContent('Will create')
  })

  it('returns to step 1 keeping the edited JSON', async () => {
    const { user } = mountDialog()
    await goToStep2(user)
    await user.click(screen.getByRole('button', { name: 'Back to editing' }))
    expect(await editor()).toHaveValue('{"new-model":{"context_length":4096}}')
    expect(screen.getByText('1. Review and edit')).toHaveAttribute(
      'aria-current',
      'step'
    )
  })

  it('blocks saving when validation fails and lists the server problems', async () => {
    const { user, post, put } = mountDialog()
    post.mockResolvedValue({ data: { success: true, data: previewWithErrors } })
    await edit('{"bad-model":{"context_length":-1}}')
    await user.click(previewButton())
    expect(
      await screen.findByText(
        '1 entries failed validation and cannot be saved now'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText('Go back and fix the JSON. You can save once everything passes.')
    ).toBeInTheDocument()
    expect(
      screen.getByText('Entries that failed validation (1)')
    ).toBeInTheDocument()
    expect(screen.getByText('bad-model')).toBeInTheDocument()
    expect(screen.getByText('context_length must be a positive integer')).toBeInTheDocument()
    expect(screen.getByText('Validation failed 1')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Replace and save/ })).toBeDisabled()
    expect(put).not.toHaveBeenCalled()
  })
})

describe('saving', () => {
  it('puts the capabilities with the preview version, then closes, toasts and refreshes', async () => {
    const { user, put, onOpenChange, invalidate } = mountDialog()
    const success = vi.spyOn(toast, 'success')
    await goToStep2(user)
    await user.click(
      screen.getByRole('button', { name: 'Replace and save 3 changes' })
    )
    await waitFor(() => expect(put).toHaveBeenCalledOnce())
    expect(put).toHaveBeenCalledWith('/api/models/capabilities', {
      capabilities: { 'new-model': { context_length: 4096 } },
      version: 'v-preview-1',
    })
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    const toasted = JSON.stringify(success.mock.calls)
    expect(toasted).toContain('Model capabilities saved')
    expect(toasted).toContain('Created 1, updated 1, cleared 1 records')
    const keys = JSON.stringify(invalidate.mock.calls)
    expect(keys).toContain('"models"')
    expect(keys).toContain('model-pricing-config')
  })

  it('disables the button and shows progress while saving', async () => {
    const { user, put } = mountDialog()
    put.mockReturnValue(new Promise(() => {}))
    await goToStep2(user)
    await user.click(screen.getByRole('button', { name: /Replace and save/ }))
    const saving = await screen.findByRole('button', { name: 'Saving...' })
    expect(saving).toBeDisabled()
  })

  it('shows the failure reason for a server error and lets the user save again', async () => {
    const { user, put, onOpenChange } = mountDialog()
    put.mockRejectedValueOnce(httpError(500, 'database busy'))
    await goToStep2(user)
    await user.click(screen.getByRole('button', { name: /Replace and save/ }))
    expect(
      await screen.findByText('Save failed. Nothing was changed.')
    ).toBeInTheDocument()
    expect(screen.getByText(/database busy/)).toBeInTheDocument()
    expect(onOpenChange).not.toHaveBeenCalled()
    const retry = screen.getByRole('button', { name: /Replace and save/ })
    expect(retry).toBeEnabled()
    await user.click(retry)
    await waitFor(() => expect(put).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })

  it('ends a non-409 save failure with a hint to try again later', async () => {
    const { user, put } = mountDialog()
    put.mockRejectedValueOnce(httpError(500, 'database busy'))
    await goToStep2(user)
    await user.click(screen.getByRole('button', { name: /Replace and save/ }))
    const alert = (
      await screen.findByText('Save failed. Nothing was changed.')
    ).closest('[data-slot="alert"]')
    expect(alert?.textContent).toMatch(
      /database busy.*You can click save again later\.$/
    )
  })

  it('on 409 asks to preview again and re-runs the preview', async () => {
    const { user, put, post } = mountDialog()
    put.mockRejectedValueOnce(httpError(409, 'stale'))
    await goToStep2(user)
    await user.click(screen.getByRole('button', { name: /Replace and save/ }))
    expect(
      await screen.findByText('Model data has changed. Nothing was saved.')
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        'After the preview, some model data was changed, so the comparison below is out of date. Preview again, confirm the latest changes, then save.'
      )
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /Replace and save/ })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByText(/You can click save again later/)
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back to editing' })).toBeEnabled()
    const previews = post.mock.calls.length
    await user.click(screen.getByRole('button', { name: 'Preview again' }))
    await waitFor(() => expect(post.mock.calls.length).toBe(previews + 1))
    expect(post).toHaveBeenLastCalledWith('/api/models/capabilities/preview', {
      capabilities: { 'new-model': { context_length: 4096 } },
    })
  })
})

describe('closing', () => {
  it('discards unsaved edits when the dialog is reopened', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: loaded },
    })
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const ui = (open: boolean) => (
      <QueryClientProvider client={client}>
        <ModelsProvider>
          <CapabilitiesJsonDialog open={open} onOpenChange={() => {}} />
        </ModelsProvider>
      </QueryClientProvider>
    )
    const view = render(ui(true))
    await edit('{"x":{"context_length":1}}')
    view.rerender(ui(false))
    view.rerender(ui(true))
    await waitFor(() => expect(get).toHaveBeenCalled())
    await waitFor(async () => expect(await editor()).toHaveValue(formatted))
  })
})

describe('locales', () => {
  const locales = ['en', 'zh', 'zh-TW', 'fr', 'ru', 'ja', 'vi']
  const keys = [
    'Capabilities JSON',
    'These are the current capabilities of all models, organized by model name, in the same format as /v1/models. You can copy or download them, edit them directly, or paste new content and save.',
    '1. Review and edit',
    '2. Preview and save',
    '{{count}} records are not listed below',
    'They are not specific model names. Edit them in the model drawer; saving here will not touch them.',
    'To configure them, add their names below.',
    "Saving replaces everything. After saving, each model's capabilities will be exactly what is written here. Fields that are omitted or set to null will be cleared. Removing a model from the JSON clears its capabilities but keeps the record. Models without a record in Model Management get a new exact-match record automatically.",
    '{{count}} models have capabilities configured',
    'You have unsaved changes',
    'Download JSON file',
    'Preview changes',
    'No model has capabilities configured yet. You can paste JSON directly.',
    'The JSON is malformed. Fix it before previewing.',
    'Failed to load model capabilities',
    'Create {{count}}',
    'Update {{count}}',
    'Clear {{count}}',
    'Unchanged {{count}}',
    'Validation failed {{count}}',
    'Capabilities of {{count}} models will be cleared',
    '{{model}} is not in the JSON. If you did not mean to remove it, go back and add it again.',
    'Field',
    'Current value',
    'Change to',
    'This model has no record in Model Management yet. An exact-match record will be created.',
    'Cleared (not configured)',
    'The model is not in the JSON, so its capabilities will be cleared. The record itself is kept.',
    'Same as the current configuration. Nothing will change.',
    'Will create {{create}}, update {{update}}, and clear {{clear}} records',
    'Back to editing',
    'Replace and save {{count}} changes',
    '{{count}} entries failed validation and cannot be saved now',
    'Go back and fix the JSON. You can save once everything passes.',
    'Entries that failed validation ({{count}})',
    'Model',
    'Problem',
    'Saving...',
    'Save failed. Nothing was changed.',
    'Model data has changed. Nothing was saved.',
    'After the preview, some model data was changed, so the comparison below is out of date. Preview again, confirm the latest changes, then save.',
    'Preview again',
    'Model capabilities saved',
    'Created {{create}}, updated {{update}}, cleared {{clear}} records',
    'Capability configuration (JSON)',
    '{{count}} records are not exact-match records: {{names}}.',
    '{{count}} records have no capabilities configured yet: {{names}}.',
    '{{name}} ({{rule}})',
    '{{count}} entries failed validation',
    'You can click save again later.',
    'To configure it, add its name below.',
  ]

  it.each(locales)('%s defines every key of the dialog', async (locale) => {
    const mod = await import(`../../../../../i18n/locales/${locale}.json`)
    const translation = (mod.default ?? mod).translation as Record<
      string,
      string
    >
    const missing = keys.filter((key) => !(key in translation))
    expect(missing).toEqual([])
  })

  it('zh saved summary has no trailing full stop', async () => {
    const mod = await import('../../../../../i18n/locales/zh.json')
    const translation = (mod.default ?? mod).translation as Record<
      string,
      string
    >
    expect(
      translation[
        'Created {{create}}, updated {{update}}, cleared {{clear}} records'
      ]
    ).not.toMatch(/[。.]$/)
  })
})

describe('zh wording of the new dialog texts', () => {
  it.each([
    ['Capability configuration (JSON)', /^能力配置（JSON）$/],
    [
      '{{count}} records are not exact-match records: {{names}}.',
      /^\{\{count\}\} ?条不是完全匹配的记录：\{\{names\}\}。$/,
    ],
    [
      '{{count}} records have no capabilities configured yet: {{names}}.',
      /^\{\{count\}\} ?条还没有配置任何能力的记录：\{\{names\}\}。$/,
    ],
    ['{{name}} ({{rule}})', /^\{\{name\}\}（\{\{rule\}\}）$/],
    [
      '{{count}} entries failed validation',
      /^\{\{count\}\} ?个条目校验没通过$/,
    ],
    ['You can click save again later.', /^可以稍后再点一次保存。$/],
    ['To configure it, add its name below.', /^要给它配置，可以在下面加上它的名字。$/],
    ['To configure them, add their names below.', /^要给它们配置，可以在下面加上它们的名字。$/],
    ['Prefix Match', /^前缀匹配$/],
    ['Suffix Match', /^后缀匹配$/],
  ])('%s', async (key, expected) => {
    const mod = await import('../../../../../i18n/locales/zh.json')
    const translation = (mod.default ?? mod).translation as Record<
      string,
      string
    >
    expect(translation[key]).toMatch(expected)
  })
})
