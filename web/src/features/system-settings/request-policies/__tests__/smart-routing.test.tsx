import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createRouter,
  createRootRoute,
  createMemoryHistory,
  RouterContextProvider,
} from '@tanstack/react-router'
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
import { useState } from 'react'
import { Toaster } from 'sonner'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { getPolicySectionNavItems } from '../section-registry'
import { SmartRoutingSection } from '../smart-routing-section'

const ROUTES_KEY = 'smart_routing_setting.routes'

const tier = (model: string, groups: string[], effort: string) => ({
  model,
  groups,
  effort,
})

const autoGpt = {
  name: 'auto-gpt',
  tiers: {
    simple: tier('gpt-mini', ['api'], 'low'),
    normal: tier('gpt-mid', ['subscription', 'api'], ''),
    hard: tier('gpt-max', ['api'], 'high'),
  },
  default_tier: 'normal',
  scorer: {
    model: 'gpt-scorer',
    groups: ['subscription', 'api'],
    timeout_ms: 2000,
  },
  advisor: { model: 'gpt-advisor', groups: ['api'], after_dissatisfied: 2 },
  downgrade_max_history_tokens: 0,
  confidence_threshold: 0.6,
  simple_max_message_chars: 200,
}

const autoChat = {
  ...autoGpt,
  name: 'auto-chat',
  tiers: {
    simple: tier('chat-mini', ['api'], 'low'),
    normal: tier('chat-mid', ['api'], 'medium'),
    hard: tier('chat-max', ['api'], 'high'),
  },
  default_tier: 'simple',
  scorer: { model: 'chat-scorer', groups: ['api'], timeout_ms: 3000 },
  advisor: { model: 'chat-advisor', groups: ['api'], after_dissatisfied: 3 },
}

const pricingModels = [
  ...['gpt-mini', 'gpt-mid', 'gpt-max', 'gpt-advisor'].map((model_name) => ({
    model_name,
    supported_endpoint_types: ['openai-response'],
  })),
  ...['chat-mini', 'chat-mid', 'chat-max', 'chat-advisor'].map(
    (model_name) => ({ model_name, supported_endpoint_types: ['openai'] })
  ),
]

let currentOptions: Record<string, string>
let pricingFails = false
let client: QueryClient

function setRoutes(routes: unknown) {
  currentOptions = { [ROUTES_KEY]: JSON.stringify(routes) }
}

function savedRoutes() {
  const payload = vi.mocked(api.patch).mock.calls[0][1] as {
    options: Record<string, string>
  }
  return JSON.parse(payload.options[ROUTES_KEY]) as { name: string }[]
}

beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  pricingFails = false
  setRoutes([autoGpt, autoChat])
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/option/request_policy') {
      return { data: { success: true, data: { options: currentOptions } } }
    }
    if (url === '/api/pricing') {
      if (pricingFails) throw new Error('pricing down')
      return {
        data: {
          success: true,
          data: pricingModels,
          vendors: [],
          group_ratio: {},
          usable_group: {},
          supported_endpoint: {},
          auto_groups: [],
        },
      }
    }
    return { data: { success: true, data: {} } }
  })
  vi.spyOn(api, 'patch').mockImplementation(async (_url, request) => {
    currentOptions = {
      ...currentOptions,
      ...(request as { options: Record<string, string> }).options,
    }
    return { data: { success: true, data: { options: currentOptions } } }
  })
})

afterEach(async () => {
  cleanup()
  client.clear()
  vi.restoreAllMocks()
  await i18next.changeLanguage('en')
})

function Workspace() {
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <Toaster />
      <div ref={setContainer} />
      <SettingsPageProvider actionsContainer={container}>
        <SmartRoutingSection />
      </SettingsPageProvider>
    </>
  )
}

function show() {
  const router = createRouter({
    routeTree: createRootRoute(),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  render(
    <QueryClientProvider client={client}>
      <RouterContextProvider router={router}>
        <Workspace />
      </RouterContextProvider>
    </QueryClientProvider>
  )
}

it('the request policy menu lists Smart routing right after Channel health', () => {
  const titles = getPolicySectionNavItems(i18next.t.bind(i18next)).map(
    (item) => item.title
  )
  expect(titles.indexOf('Smart routing')).toBe(
    titles.indexOf('Channel health') + 1
  )
})

it('an empty list shows the empty state with an add button', async () => {
  setRoutes([])
  show()
  expect(await screen.findByText('No virtual models yet')).toBeVisible()
  expect(
    screen.getByText(
      'Add a virtual model such as auto-gpt. Users enter it as the model name in their client.'
    )
  ).toBeVisible()
  expect(
    screen.getAllByRole('button', { name: 'Add virtual model' }).length
  ).toBeGreaterThan(0)
})

it('renders the count badge and two-line cells for every virtual model', async () => {
  show()
  const table = await screen.findByRole('table')
  expect(screen.getByText('Virtual models')).toBeVisible()
  expect(screen.getByText('2')).toBeVisible()
  for (const header of [
    'Name',
    'Simple tier',
    'Normal tier',
    'Hard tier',
    'Scorer model',
    'Advisor',
  ]) {
    expect(
      within(table).getByRole('columnheader', { name: header })
    ).toBeVisible()
  }

  const gpt = await screen.findByRole('row', { name: /auto-gpt/ })
  expect(gpt).toHaveTextContent('Responses endpoint')
  expect(gpt).toHaveTextContent('api · low')
  expect(gpt).toHaveTextContent('subscription → api · Keep client value')
  expect(gpt).toHaveTextContent('api · high')
  expect(gpt).toHaveTextContent('subscription → api · Timeout 2000 ms')
  expect(gpt).toHaveTextContent('After 2 dissatisfied messages')
  expect(within(gpt).getAllByText('Default')).toHaveLength(1)
  expect(gpt).toHaveTextContent(/gpt-mid\s*Default/)

  const chat = screen.getByRole('row', { name: /auto-chat/ })
  expect(chat).toHaveTextContent('Chat Completions endpoint')
  expect(chat).toHaveTextContent('api · Timeout 3000 ms')
  expect(chat).toHaveTextContent('After 3 dissatisfied messages')
  expect(chat).toHaveTextContent(/chat-mini\s*Default/)
})

it('deleting asks for confirmation and the saved array omits the row', async () => {
  show()
  const row = await screen.findByRole('row', { name: /auto-gpt/ })
  await userEvent.click(
    within(row).getByRole('button', { name: 'More actions' })
  )
  await userEvent.click(screen.getByRole('menuitem', { name: 'Delete' }))
  const dialog = screen.getByRole('alertdialog')
  expect(dialog).toHaveTextContent('Delete virtual model')
  expect(dialog).toHaveTextContent(
    'Delete auto-gpt? It takes effect after you save changes.'
  )
  expect(api.patch).not.toHaveBeenCalled()
  await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
  expect(
    screen.queryByRole('row', { name: /auto-gpt/ })
  ).not.toBeInTheDocument()
  expect(api.patch).not.toHaveBeenCalled()

  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(api.patch).toHaveBeenCalledTimes(1))
  expect(vi.mocked(api.patch).mock.calls[0][0]).toBe(
    '/api/option/request_policy'
  )
  expect(savedRoutes().map((route) => route.name)).toEqual(['auto-chat'])
})

it('saving without changes shows a notice and sends no request', async () => {
  show()
  await screen.findByRole('table')
  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(await screen.findByText('No changes to save')).toBeVisible()
  expect(api.patch).not.toHaveBeenCalled()
})

it('a non-array JSON draft reports an error and stays in the JSON view', async () => {
  show()
  await userEvent.click(await screen.findByRole('tab', { name: 'JSON' }))
  const editor = await screen.findByRole('textbox')
  fireEvent.input(editor, { target: { value: '{"name":"x"}' } })
  await userEvent.click(screen.getByRole('tab', { name: 'Visual' }))
  expect(await screen.findByText(/must be an array/i)).toBeVisible()
  expect(screen.getByRole('tab', { name: 'JSON' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  expect(api.patch).not.toHaveBeenCalled()
})

it('a rejected save shows the backend message and keeps the edit', async () => {
  vi.mocked(api.patch).mockResolvedValue({
    data: { success: false, message: 'Save rejected' },
  })
  show()
  const row = await screen.findByRole('row', { name: /auto-gpt/ })
  await userEvent.click(
    within(row).getByRole('button', { name: 'More actions' })
  )
  await userEvent.click(screen.getByRole('menuitem', { name: 'Delete' }))
  await userEvent.click(
    within(screen.getByRole('alertdialog')).getByRole('button', {
      name: 'Delete',
    })
  )
  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(await screen.findByText('Save rejected')).toBeVisible()
  expect(
    screen.queryByRole('row', { name: /auto-gpt/ })
  ).not.toBeInTheDocument()
})

function routeWith(name: string, prefix: string) {
  return {
    ...autoGpt,
    name,
    tiers: {
      simple: tier(`${prefix}-a`, ['api'], 'low'),
      normal: tier(`${prefix}-b`, ['api'], 'low'),
      hard: tier(`${prefix}-c`, ['api'], 'low'),
    },
    advisor: { model: `${prefix}-d`, groups: ['api'], after_dissatisfied: 2 },
  }
}

function addPricing(prefix: string, endpoints: string[]) {
  for (const suffix of ['a', 'b', 'c', 'd']) {
    pricingModels.push({
      model_name: `${prefix}-${suffix}`,
      supported_endpoint_types: endpoints,
    })
  }
}

async function writeJson(value: string) {
  await userEvent.click(await screen.findByRole('tab', { name: 'JSON' }))
  const editor = await screen.findByRole('textbox')
  fireEvent.input(editor, { target: { value } })
  await userEvent.click(screen.getByRole('tab', { name: 'Visual' }))
  return editor
}

it('shows both endpoints with Responses first, and ignores anthropic and gemini', async () => {
  addPricing('both', ['openai', 'openai-response'])
  addPricing('claude', ['anthropic', 'openai-response', 'gemini'])
  setRoutes([
    routeWith('auto-both', 'both'),
    routeWith('auto-claude', 'claude'),
  ])
  show()
  await screen.findByRole('row', { name: /auto-both/ })
  await waitFor(() =>
    expect(screen.getByRole('row', { name: /auto-both/ })).toHaveTextContent(
      'Responses / Chat Completions endpoint'
    )
  )
  const claude = screen.getByRole('row', { name: /auto-claude/ })
  expect(claude).toHaveTextContent('Responses endpoint')
  expect(claude).not.toHaveTextContent(/anthropic|gemini/i)
})

it.each(['[{"name":"x"}]', '[null]'])(
  'malformed JSON %s reports an error, stays in JSON view and keeps the text',
  async (value) => {
    show()
    const editor = await writeJson(value)
    expect(
      await screen.findByText(/must be|must include|invalid/i)
    ).toBeVisible()
    expect(screen.getByRole('tab', { name: 'JSON' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(editor).toBeInTheDocument()
    expect(editor).toHaveValue(value)
  }
)

it('deleting one of two same-named records keeps the other', async () => {
  show()
  await writeJson(
    JSON.stringify(
      [autoGpt, { ...autoGpt }].map((r) => ({ ...r, name: 'dup' }))
    )
  )
  const rows = await screen.findAllByRole('row', { name: /dup/ })
  await userEvent.click(
    within(rows[0]).getByRole('button', { name: 'More actions' })
  )
  await userEvent.click(screen.getByRole('menuitem', { name: 'Delete' }))
  await userEvent.click(
    within(screen.getByRole('alertdialog')).getByRole('button', {
      name: 'Delete',
    })
  )
  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(api.patch).toHaveBeenCalledTimes(1))
  expect(savedRoutes().map((route) => route.name)).toEqual(['dup'])
})

it('after a successful save a second save reports no changes and sends only the routes key', async () => {
  show()
  const row = await screen.findByRole('row', { name: /auto-gpt/ })
  await userEvent.click(
    within(row).getByRole('button', { name: 'More actions' })
  )
  await userEvent.click(screen.getByRole('menuitem', { name: 'Delete' }))
  await userEvent.click(
    within(screen.getByRole('alertdialog')).getByRole('button', {
      name: 'Delete',
    })
  )
  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(api.patch).toHaveBeenCalledTimes(1))
  const payload = vi.mocked(api.patch).mock.calls[0][1] as {
    options: Record<string, string>
  }
  expect(Object.keys(payload.options)).toEqual([ROUTES_KEY])
  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(await screen.findByText('No changes to save')).toBeVisible()
  expect(api.patch).toHaveBeenCalledTimes(1)
})

it('a failing pricing request still renders the table without endpoint lines', async () => {
  pricingFails = true
  show()
  const row = await screen.findByRole('row', { name: /auto-gpt/ })
  expect(row).not.toHaveTextContent(/endpoint/)
  expect(screen.getByRole('row', { name: /auto-chat/ })).not.toHaveTextContent(
    /endpoint/
  )
})

it('each row has an actions group named after the virtual model', async () => {
  show()
  await screen.findByRole('table')
  expect(
    screen.getByRole('group', { name: 'Actions for auto-gpt' })
  ).toBeVisible()
  expect(
    screen.getByRole('group', { name: 'Actions for auto-chat' })
  ).toBeVisible()
})

const dialogRoute = { ...autoGpt, downgrade_max_history_tokens: 20000 }
const validationError = (field: string, model: string) => ({
  field,
  message: {
    key: '{{model}} has no available channel in group {{group}}',
    params: { model, group: 'api' },
  },
})

function mockDialogApis(validateResult: unknown) {
  const base = vi.mocked(api.get).getMockImplementation()
  if (!base) throw new Error('api.get is not mocked')
  vi.mocked(api.get).mockImplementation(async (url, config) => {
    if (url === '/api/channel/models_enabled') {
      return {
        data: {
          success: true,
          data: [
            ...pricingModels.map((model) => model.model_name),
            'gpt-scorer',
          ],
        },
      }
    }
    if (url === '/api/group/') {
      return { data: { success: true, data: ['api', 'subscription'] } }
    }
    return base(url, config)
  })
  return vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: validateResult },
  })
}

async function openEditDialog(name: string) {
  const row = await screen.findByRole('row', { name: new RegExp(name) })
  await userEvent.click(within(row).getByRole('button', { name: 'Edit' }))
  return screen.findByRole('dialog')
}

async function saveDialog() {
  await userEvent.click(
    within(screen.getByRole('dialog')).getByRole('button', { name: 'Save' })
  )
}

async function pickModel(label: string, model: string) {
  await userEvent.click(screen.getByLabelText(label))
  await userEvent.click(await screen.findByRole('option', { name: model }))
}

it('the add dialog starts with the documented defaults', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Add virtual model' })
  )
  const dialog = await screen.findByRole('dialog')
  expect(dialog).toHaveTextContent('Add virtual model')
  expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('')
  expect(within(dialog).getByLabelText('Interface')).toHaveAttribute(
    'placeholder',
    'Detected after you pick models'
  )
  expect(within(dialog).getAllByText('Keep client value').length).toBe(3)
  expect(within(dialog).getByLabelText('Default tier')).toHaveTextContent(
    'Normal tier'
  )
  expect(within(dialog).getByLabelText('Timeout (ms)')).toHaveValue(2000)
  expect(
    within(dialog).getByLabelText(
      'Dissatisfied messages before the advisor answers'
    )
  ).toHaveValue(2)
  expect(
    within(dialog).getByLabelText('Downgrade history token limit')
  ).toHaveValue(20000)
  expect(within(dialog).getByLabelText('Confidence threshold')).toHaveValue(0.6)
  expect(
    within(dialog).getByLabelText('Simple message length limit (characters)')
  ).toHaveValue(2000)
})

it('the edit icon opens the dialog with the saved values', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openEditDialog('auto-chat')
  expect(dialog).toHaveTextContent('Edit virtual model')
  expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('auto-chat')
  expect(within(dialog).getByLabelText('Simple tier model')).toHaveValue(
    'chat-mini'
  )
  expect(within(dialog).getByLabelText('Hard tier model')).toHaveValue(
    'chat-max'
  )
  expect(within(dialog).getByLabelText('Default tier')).toHaveTextContent(
    'Simple tier'
  )
  expect(within(dialog).getByLabelText('Scorer model')).toHaveValue(
    'chat-scorer'
  )
  expect(within(dialog).getByLabelText('Timeout (ms)')).toHaveValue(3000)
  expect(
    within(dialog).getByLabelText(
      'Dissatisfied messages before the advisor answers'
    )
  ).toHaveValue(3)
})

it('an empty name and a 50 ms timeout fail locally without calling the check endpoint', async () => {
  const post = mockDialogApis({ errors: [], endpoints: [] })
  show()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Add virtual model' })
  )
  const dialog = await screen.findByRole('dialog')
  const timeout = within(dialog).getByLabelText('Timeout (ms)')
  await userEvent.clear(timeout)
  await userEvent.type(timeout, '50')
  await saveDialog()
  await waitFor(() =>
    expect(within(dialog).getByLabelText(/^Name/)).toHaveAttribute(
      'aria-invalid',
      'true'
    )
  )
  expect(timeout).toHaveAttribute('aria-invalid', 'true')
  expect(post).not.toHaveBeenCalled()
  expect(screen.getByRole('dialog')).toBeVisible()
})

it('server errors appear under their fields, translated, and the dialog stays open', async () => {
  setRoutes([dialogRoute, autoChat])
  const post = mockDialogApis({
    errors: [
      validationError('tiers.hard.model', 'gpt-max'),
      validationError('scorer.model', 'gpt-scorer'),
    ],
    endpoints: [],
  })
  show()
  const dialog = await openEditDialog('auto-gpt')
  await saveDialog()
  expect(
    await within(dialog).findByText(
      'gpt-max has no available channel in group api'
    )
  ).toBeVisible()
  expect(
    within(dialog).getByText('gpt-scorer has no available channel in group api')
  ).toBeVisible()
  expect(post).toHaveBeenCalledTimes(1)
  expect(post.mock.calls[0][0]).toBe('/api/option/smart_routing/validate')
  expect(post.mock.calls[0][1]).toMatchObject({
    route: { name: 'auto-gpt' },
    original_name: 'auto-gpt',
  })
  expect(screen.getByRole('dialog')).toBeVisible()
  expect(api.patch).not.toHaveBeenCalled()
})

it('an error on an advanced field expands the advanced settings', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({
    errors: [
      {
        field: 'confidence_threshold',
        message: {
          key: 'Enter a value between {{min}} and {{max}}',
          params: { min: 0, max: 1 },
        },
      },
    ],
    endpoints: [],
  })
  show()
  const dialog = await openEditDialog('auto-gpt')
  const toggle = within(dialog).getByRole('button', {
    name: 'Advanced settings',
  })
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await saveDialog()
  expect(
    await within(dialog).findByText('Enter a value between 0 and 1')
  ).toBeVisible()
  expect(toggle).toHaveAttribute('aria-expanded', 'true')
})

it('a passing check closes the dialog and the renamed row is saved with the page', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({ errors: [], endpoints: ['openai-response'] })
  show()
  const dialog = await openEditDialog('auto-gpt')
  const name = within(dialog).getByLabelText(/^Name/)
  await userEvent.clear(name)
  await userEvent.type(name, 'auto-gpt-v2')
  await saveDialog()
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(vi.mocked(api.post).mock.calls[0][1]).toMatchObject({
    original_name: 'auto-gpt',
  })
  expect(screen.getByRole('row', { name: /auto-gpt-v2/ })).toBeVisible()
  expect(api.patch).not.toHaveBeenCalled()

  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(api.patch).toHaveBeenCalledTimes(1))
  expect(savedRoutes().map((route) => route.name)).toEqual([
    'auto-gpt-v2',
    'auto-chat',
  ])
})

it('a new virtual model passes the check with an empty original name and joins the table', async () => {
  const post = mockDialogApis({ errors: [], endpoints: ['openai-response'] })
  show()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Add virtual model' })
  )
  const dialog = await screen.findByRole('dialog')
  await userEvent.type(within(dialog).getByLabelText(/^Name/), 'auto-new')
  await pickModel('Simple tier model', 'gpt-mini')
  await pickModel('Normal tier model', 'gpt-mid')
  await pickModel('Hard tier model', 'gpt-max')
  await pickModel('Scorer model', 'gpt-scorer')
  await pickModel('Advisor', 'gpt-advisor')
  for (let i = 0; i < 5; i++) {
    await userEvent.click(
      within(dialog).getAllByRole('combobox', { name: 'Add group' })[i]
    )
    await userEvent.click(await screen.findByRole('option', { name: /api/ }))
  }
  await saveDialog()
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(post.mock.calls[0][1]).toMatchObject({
    route: { name: 'auto-new', default_tier: 'normal' },
    original_name: '',
  })
  expect(screen.getByRole('row', { name: /auto-new/ })).toBeVisible()

  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(api.patch).toHaveBeenCalledTimes(1))
  expect(savedRoutes().map((route) => route.name)).toEqual([
    'auto-gpt',
    'auto-chat',
    'auto-new',
  ])
})

it('saved efforts are low for a picked value and empty for Keep client value', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openEditDialog('auto-gpt')
  const efforts = within(dialog).getAllByLabelText('Reasoning effort')
  await userEvent.click(efforts[1])
  await userEvent.click(
    await screen.findByRole('option', { name: 'Keep client value' })
  )
  await userEvent.click(within(dialog).getAllByLabelText('Reasoning effort')[0])
  await userEvent.click(await screen.findByRole('option', { name: 'Low' }))
  await saveDialog()
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  const route = vi.mocked(api.post).mock.calls[0][1] as {
    route: { tiers: Record<string, { effort: string }> }
  }
  expect(route.route.tiers.simple.effort).toBe('low')
  expect(route.route.tiers.normal.effort).toBe('')
})

it('effort and default tier server errors mark their fields', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({
    errors: [
      {
        field: 'tiers.hard.effort',
        message: { key: 'Select a valid reasoning effort' },
      },
      { field: 'default_tier', message: { key: 'Select a default tier' } },
    ],
    endpoints: [],
  })
  show()
  const dialog = await openEditDialog('auto-gpt')
  await saveDialog()
  expect(
    await within(dialog).findByText('Select a valid reasoning effort')
  ).toBeVisible()
  expect(within(dialog).getByText('Select a default tier')).toBeVisible()
  expect(
    within(dialog).getAllByLabelText('Reasoning effort')[2]
  ).toHaveAttribute('aria-invalid', 'true')
  expect(within(dialog).getByLabelText('Default tier')).toHaveAttribute(
    'aria-invalid',
    'true'
  )
})

it('a local failure on an advanced field expands the advanced settings without calling the check', async () => {
  setRoutes([dialogRoute, autoChat])
  const post = mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openEditDialog('auto-gpt')
  const toggle = within(dialog).getByRole('button', {
    name: 'Advanced settings',
  })
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await userEvent.click(toggle)
  const threshold = within(dialog).getByLabelText('Confidence threshold')
  await userEvent.clear(threshold)
  await userEvent.type(threshold, '2')
  await userEvent.click(toggle)
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await saveDialog()
  await waitFor(() => expect(toggle).toHaveAttribute('aria-expanded', 'true'))
  expect(within(dialog).getByLabelText('Confidence threshold')).toHaveAttribute(
    'aria-invalid',
    'true'
  )
  expect(post).not.toHaveBeenCalled()
})

it('a stored virtual model missing effort and history limit still reaches the check', async () => {
  const post = mockDialogApis({ errors: [], endpoints: [] })
  show()
  const { downgrade_max_history_tokens: _drop, ...rest } = dialogRoute
  const legacy = {
    ...rest,
    tiers: {
      simple: { model: 'gpt-mini', groups: ['api'] },
      normal: { model: 'gpt-mid', groups: ['api'] },
      hard: { model: 'gpt-max', groups: ['api'] },
    },
  }
  await writeJson(JSON.stringify([legacy]))
  await openEditDialog('auto-gpt')
  await saveDialog()
  await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
  const sent = post.mock.calls[0][1] as {
    route: {
      downgrade_max_history_tokens: number
      tiers: Record<string, { effort: string }>
    }
  }
  expect(sent.route.downgrade_max_history_tokens).toBe(0)
  expect(sent.route.tiers.simple.effort).toBe('')
})

it('cancelling while the check is pending leaves the table unchanged', async () => {
  setRoutes([dialogRoute, autoChat])
  let release: (value: unknown) => void = () => {}
  mockDialogApis({ errors: [], endpoints: [] })
  vi.mocked(api.post).mockImplementation(
    () =>
      new Promise((resolve) => {
        release = resolve
      })
  )
  show()
  const dialog = await openEditDialog('auto-gpt')
  const name = within(dialog).getByLabelText(/^Name/)
  await userEvent.clear(name)
  await userEvent.type(name, 'auto-renamed')
  await saveDialog()
  await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1))
  await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  release({ data: { success: true, data: { errors: [], endpoints: [] } } })
  await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1))
  expect(screen.queryByRole('row', { name: /auto-renamed/ })).toBeNull()
  expect(screen.getByRole('row', { name: /auto-gpt/ })).toBeVisible()
})
