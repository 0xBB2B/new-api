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

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import viLocale from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { getPolicySectionNavItems } from '../section-registry'
import { SmartRoutingSection } from '../smart-routing-section'

const ROUTES_KEY = 'smart_routing_setting.routes'

type TierOptions = {
  effort_mode?: string
  effort?: string
  effort_min?: string
  effort_max?: string
}

const tier = (model: string, groups: string[], options: TierOptions = {}) => ({
  model,
  groups,
  effort_mode: '',
  effort: '',
  effort_min: '',
  effort_max: '',
  ...options,
})

const autoGpt = {
  name: 'auto-gpt',
  endpoints: ['openai-response', 'anthropic'],
  tiers: {
    simple: tier('gpt-mini', ['api']),
    normal: tier('gpt-mid', ['subscription', 'api'], {
      effort_mode: 'fixed',
      effort: 'medium',
    }),
    hard: tier('gpt-max', ['api'], {
      effort_mode: 'follow',
      effort_min: 'medium',
      effort_max: 'xhigh',
    }),
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
  endpoints: ['openai'],
  tiers: {
    simple: tier('gpt-4.1', ['api']),
    normal: tier('glm-5.1', ['api']),
    hard: tier('chat-max', ['api']),
  },
  default_tier: 'normal',
  scorer: { model: 'chat-scorer', groups: ['api'], timeout_ms: 3000 },
  advisor: { model: 'chat-advisor', groups: ['api'], after_dissatisfied: 3 },
}

const enabledModels = [
  'gpt-mini',
  'gpt-mid',
  'gpt-max',
  'gpt-advisor',
  'gpt-scorer',
  'gpt-4.1',
  'glm-5.1',
  'chat-max',
  'chat-advisor',
  'chat-scorer',
]

const builtinCapabilities: Record<
  string,
  { known: boolean; no_thinking: boolean; efforts: string[] }
> = {
  'gpt-mini': {
    known: true,
    no_thinking: false,
    efforts: ['low', 'medium', 'high'],
  },
  'gpt-mid': {
    known: true,
    no_thinking: false,
    efforts: ['none', 'low', 'medium', 'high'],
  },
  'gpt-max': {
    known: true,
    no_thinking: false,
    efforts: ['low', 'medium', 'high', 'xhigh'],
  },
  'gpt-4.1': { known: true, no_thinking: true, efforts: [] },
}

let currentOptions: Record<string, string>
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
  setRoutes([autoGpt, autoChat])
  vi.spyOn(api, 'get').mockImplementation(async (url, config) => {
    if (url === '/api/option/request_policy') {
      return { data: { success: true, data: { options: currentOptions } } }
    }
    if (url === '/api/pricing') {
      return {
        data: {
          success: true,
          data: enabledModels.map((model_name) => ({
            model_name,
            supported_endpoint_types: [],
          })),
          vendors: [],
          group_ratio: {},
          usable_group: {},
          supported_endpoint: {},
          auto_groups: [],
        },
      }
    }
    if (url === '/api/option/smart_routing/thinking_capabilities') {
      const asked = String(
        (config as { params: { models: string } }).params.models
      ).split(',')
      return {
        data: {
          success: true,
          data: Object.fromEntries(
            asked.map((model) => [
              model,
              builtinCapabilities[model] ?? {
                known: false,
                no_thinking: false,
                efforts: [],
              },
            ])
          ),
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
  expect(gpt).toHaveTextContent('Responses / Anthropic')
  expect(gpt).toHaveTextContent('Not overridden')
  expect(gpt).toHaveTextContent('Fixed medium')
  expect(gpt).toHaveTextContent('Follow scoring medium～xhigh')
  expect(gpt).toHaveTextContent('subscription → api')
  expect(gpt).toHaveTextContent('subscription → api · Timeout 2000 ms')
  expect(gpt).toHaveTextContent('After 2 dissatisfied messages')
  expect(within(gpt).getAllByText('Default')).toHaveLength(1)
  expect(gpt).toHaveTextContent(/gpt-mid\s*Default/)

  const chat = screen.getByRole('row', { name: /auto-chat/ })
  expect(chat).toHaveTextContent('Chat')
  expect(chat).not.toHaveTextContent('Responses')
  expect(chat).toHaveTextContent('api · Timeout 3000 ms')
  expect(chat).toHaveTextContent('After 3 dissatisfied messages')
  expect(chat).toHaveTextContent(/glm-5.1\s*Default/)
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

async function writeJson(value: string) {
  await userEvent.click(await screen.findByRole('tab', { name: 'JSON' }))
  const editor = await screen.findByRole('textbox')
  fireEvent.input(editor, { target: { value } })
  await userEvent.click(screen.getByRole('tab', { name: 'Visual' }))
  return editor
}

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
          data: enabledModels,
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
  await userEvent.click(screen.getByRole('combobox', { name: label }))
  await userEvent.click(await screen.findByRole('option', { name: model }))
}

async function openTab(name: RegExp) {
  await userEvent.click(screen.getByRole('tab', { name }))
}

function expectSelectedTab(name: RegExp) {
  expect(screen.getByRole('tab', { name })).toHaveAttribute(
    'aria-selected',
    'true'
  )
}

async function addGroup(group: string) {
  const trigger =
    screen.queryByRole('combobox', { name: 'Add group' }) ??
    screen.getByRole('button', { name: /Add group/ })
  await userEvent.click(trigger)
  await userEvent.click(
    await screen.findByRole('option', { name: new RegExp(group) })
  )
}

async function fillNewRoute(name: string) {
  const dialog = await screen.findByRole('dialog')
  await userEvent.type(within(dialog).getByLabelText(/^Name/), name)
  await pickModel('Scorer model', 'gpt-scorer')
  await addGroup('api')
  await openTab(/^Tiers/)
  for (const [tabName, label, model] of [
    [/^Simple/, 'Simple tier model', 'gpt-mini'],
    [/^Normal/, 'Normal tier model', 'gpt-mid'],
    [/^Hard/, 'Hard tier model', 'gpt-max'],
  ] as const) {
    await openTab(tabName)
    await pickModel(label, model)
    await addGroup('api')
  }
  await openTab(/^Advisor/)
  await pickModel('Advisor model', 'gpt-advisor')
  await addGroup('api')
}

async function openTierPanel(routeName: string, tierTab: RegExp) {
  const dialog = await openEditDialog(routeName)
  await openTab(/^Tiers/)
  await openTab(tierTab)
  return dialog
}

function effortOptionNames() {
  return screen.getAllByRole('option').map((option) => option.textContent)
}

function sentRoute() {
  return (
    vi.mocked(api.post).mock.calls[0][1] as {
      route: {
        endpoints: string[]
        tiers: Record<
          string,
          {
            effort_mode: string
            effort: string
            effort_min: string
            effort_max: string
          }
        >
      }
    }
  ).route
}

it('a new virtual model opens on Scoring with only Responses pressed and nothing set', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Add virtual model' })
  )
  const dialog = await screen.findByRole('dialog')
  expect(dialog).toHaveTextContent('Add virtual model')
  expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('')
  expect(
    within(dialog).getByRole('button', { name: 'Responses' })
  ).toHaveAttribute('aria-pressed', 'true')
  expect(within(dialog).getByRole('button', { name: 'Chat' })).toHaveAttribute(
    'aria-pressed',
    'false'
  )
  expect(
    within(dialog).getByRole('button', { name: 'Anthropic' })
  ).toHaveAttribute('aria-pressed', 'false')
  expectSelectedTab(/^Scoring/)
  expect(screen.getByRole('tab', { name: /^Scoring/ })).toHaveTextContent(
    'Not set'
  )
  expect(screen.getByRole('tab', { name: /^Tiers/ })).toHaveTextContent(
    '0/3 set'
  )
  expect(screen.getByRole('tab', { name: /^Advisor/ })).toHaveTextContent(
    'Not set'
  )
  expect(screen.getByRole('combobox', { name: 'Scorer model' })).toHaveValue('')
  expect(
    screen.getByRole('combobox', { name: 'Add group' })
  ).toBeInTheDocument()
  expect(screen.queryByText('subscription')).toBeNull()
})

it('a new virtual model starts with the documented default values', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Add virtual model' })
  )
  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByLabelText('Timeout (ms)')).toHaveValue(2000)
  expect(
    within(dialog).getByLabelText('Downgrade history token limit')
  ).toHaveValue(20000)
  expect(within(dialog).getByLabelText('Confidence threshold')).toHaveValue(0.6)
  expect(
    within(dialog).getByLabelText('Simple message length limit (characters)')
  ).toHaveValue(2000)
  await openTab(/^Tiers/)
  expect(
    screen.getByRole('combobox', { name: 'Default tier' })
  ).toHaveTextContent('Normal')
  await openTab(/^Advisor/)
  expect(
    within(dialog).getByLabelText(
      'Dissatisfied messages before the advisor answers'
    )
  ).toHaveValue(2)
})

it('editing opens on Scoring with the saved interfaces and a summary on every left tab', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openEditDialog('auto-gpt')
  expect(dialog).toHaveTextContent('Edit virtual model')
  expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('auto-gpt')
  expectSelectedTab(/^Scoring/)
  expect(screen.getByRole('combobox', { name: 'Scorer model' })).toHaveValue(
    'gpt-scorer'
  )
  expect(within(dialog).getByLabelText('Timeout (ms)')).toHaveValue(2000)
  expect(
    within(dialog).getByRole('button', { name: 'Responses' })
  ).toHaveAttribute('aria-pressed', 'true')
  expect(
    within(dialog).getByRole('button', { name: 'Anthropic' })
  ).toHaveAttribute('aria-pressed', 'true')
  expect(within(dialog).getByRole('button', { name: 'Chat' })).toHaveAttribute(
    'aria-pressed',
    'false'
  )
  expect(screen.getByRole('tab', { name: /^Scoring/ })).toHaveTextContent(
    'gpt-scorer'
  )
  expect(screen.getByRole('tab', { name: /^Tiers/ })).toHaveTextContent(
    '3/3 set'
  )
  expect(screen.getByRole('tab', { name: /^Advisor/ })).toHaveTextContent(
    'gpt-advisor'
  )
})

it('the Tiers tab shows each tier with its model and marks only the default tier', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openEditDialog('auto-gpt')
  await openTab(/^Tiers/)
  const simple = screen.getByRole('tab', { name: /^Simple/ })
  const normal = screen.getByRole('tab', { name: /^Normal/ })
  const hard = screen.getByRole('tab', { name: /^Hard/ })
  expect(simple).toHaveTextContent('gpt-mini')
  expect(normal).toHaveTextContent('gpt-mid')
  expect(hard).toHaveTextContent('gpt-max')
  expect(normal).toHaveTextContent('Default')
  expect(simple).not.toHaveTextContent('Default')
  expect(hard).not.toHaveTextContent('Default')
  expect(
    screen.getByRole('combobox', { name: 'Default tier' })
  ).toHaveTextContent('Normal')
})

it('the default tier selector moves the Default label to the chosen tier', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openEditDialog('auto-gpt')
  await openTab(/^Tiers/)
  await userEvent.click(screen.getByRole('combobox', { name: 'Default tier' }))
  await userEvent.click(await screen.findByRole('option', { name: /Hard/ }))
  expect(screen.getByRole('tab', { name: /^Hard/ })).toHaveTextContent(
    'Default'
  )
  expect(screen.getByRole('tab', { name: /^Normal/ })).not.toHaveTextContent(
    'Default'
  )
})

it('a tier that follows scoring shows its range and offers only the levels the model supports', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openTierPanel('auto-gpt', /^Hard/)
  expect(screen.getByRole('combobox', { name: 'Hard tier model' })).toHaveValue(
    'gpt-max'
  )
  expect(
    screen.getByRole('button', { name: 'Follow scoring' })
  ).toHaveAttribute('aria-pressed', 'true')
  expect(
    screen.getByRole('button', { name: 'Not overridden' })
  ).toHaveAttribute('aria-pressed', 'false')
  const minimum = screen.getByRole('combobox', { name: 'Minimum effort' })
  const maximum = screen.getByRole('combobox', { name: 'Maximum effort' })
  let range = minimum.parentElement
  while (range && !range.contains(maximum)) range = range.parentElement
  expect(range).toHaveTextContent('～')
  expect(minimum).toHaveTextContent('medium')
  expect(maximum).toHaveTextContent('xhigh')
  await userEvent.click(minimum)
  expect(effortOptionNames()).toEqual(['low', 'medium', 'high', 'xhigh'])
})

it('a fixed tier shows one dropdown with the levels of its model', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openTierPanel('auto-gpt', /^Normal/)
  expect(screen.getByRole('button', { name: 'Fixed' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  const effort = screen.getByRole('combobox', { name: 'Reasoning effort' })
  expect(effort).toHaveTextContent('medium')
  expect(screen.queryByRole('combobox', { name: 'Minimum effort' })).toBeNull()
  await userEvent.click(effort)
  expect(effortOptionNames()).toEqual(['none', 'low', 'medium', 'high'])
})

it('switching from follow scoring to Fixed leaves one dropdown and submits empty range fields', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openTierPanel('auto-gpt', /^Hard/)
  await userEvent.click(screen.getByRole('button', { name: 'Fixed' }))
  expect(screen.queryByRole('combobox', { name: 'Minimum effort' })).toBeNull()
  expect(screen.queryByRole('combobox', { name: 'Maximum effort' })).toBeNull()
  await userEvent.click(
    screen.getByRole('combobox', { name: 'Reasoning effort' })
  )
  await userEvent.click(await screen.findByRole('option', { name: 'high' }))
  await saveDialog()
  await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1))
  expect(sentRoute().tiers.hard).toMatchObject({
    effort_mode: 'fixed',
    effort: 'high',
    effort_min: '',
    effort_max: '',
  })
})

it('switching from Fixed to follow scoring submits an empty fixed effort', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openTierPanel('auto-gpt', /^Normal/)
  await userEvent.click(screen.getByRole('button', { name: 'Follow scoring' }))
  expect(
    screen.queryByRole('combobox', { name: 'Reasoning effort' })
  ).toBeNull()
  await userEvent.click(
    screen.getByRole('combobox', { name: 'Minimum effort' })
  )
  await userEvent.click(await screen.findByRole('option', { name: 'low' }))
  await userEvent.click(
    screen.getByRole('combobox', { name: 'Maximum effort' })
  )
  await userEvent.click(await screen.findByRole('option', { name: 'high' }))
  await saveDialog()
  await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1))
  expect(sentRoute().tiers.normal).toMatchObject({
    effort_mode: 'follow',
    effort: '',
    effort_min: 'low',
    effort_max: 'high',
  })
})

it('switching to Not overridden removes every dropdown and submits empty effort fields', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openTierPanel('auto-gpt', /^Hard/)
  await userEvent.click(screen.getByRole('button', { name: 'Not overridden' }))
  expect(
    screen.queryByRole('combobox', { name: 'Reasoning effort' })
  ).toBeNull()
  expect(screen.queryByRole('combobox', { name: 'Minimum effort' })).toBeNull()
  expect(screen.queryByRole('combobox', { name: 'Maximum effort' })).toBeNull()
  await saveDialog()
  await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1))
  expect(sentRoute().tiers.hard).toMatchObject({
    effort_mode: '',
    effort: '',
    effort_min: '',
    effort_max: '',
  })
})

it('a model without thinking shows a grey notice and no effort controls', async () => {
  setRoutes([autoGpt, autoChat])
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openTierPanel('auto-chat', /^Simple/)
  expect(
    await within(dialog).findByText(
      'This model does not support reasoning effort. No reasoning parameter is sent with the request'
    )
  ).toBeVisible()
  expect(screen.queryByRole('button', { name: 'Fixed' })).toBeNull()
  expect(
    screen.queryByRole('combobox', { name: 'Reasoning effort' })
  ).toBeNull()
  expect(screen.queryByRole('combobox', { name: 'Minimum effort' })).toBeNull()
})

it('a model with unconfirmed capability offers low, medium and high and shows an unconfirmed tag', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openTierPanel('auto-chat', /^Normal/)
  expect(
    await within(dialog).findByText('Capability not confirmed')
  ).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: 'Fixed' }))
  await userEvent.click(
    screen.getByRole('combobox', { name: 'Reasoning effort' })
  )
  expect(effortOptionNames()).toEqual(['low', 'medium', 'high'])
})

it('the unconfirmed tag sits on the reasoning effort row, can be focused and explains itself', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openTierPanel('auto-chat', /^Normal/)
  const badge = await within(dialog).findByText('Capability not confirmed')
  let row = screen.getByRole('button', { name: 'Fixed' }).parentElement
  while (row && !row.contains(badge)) row = row.parentElement
  expect(row).not.toContainElement(
    screen.getByRole('combobox', { name: 'Normal tier model' })
  )
  const trigger = badge.closest<HTMLElement>('[tabindex]')
  expect(trigger).not.toBeNull()
  expect(trigger).not.toHaveAttribute('tabindex', '-1')
  trigger?.focus()
  expect((await screen.findAllByRole('tooltip')).length).toBeGreaterThan(0)
})

it('a confirmed model does not show the unconfirmed tag', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openTierPanel('auto-gpt', /^Normal/)
  expect(within(dialog).getByRole('button', { name: 'Fixed' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  expect(within(dialog).queryByText('Capability not confirmed')).toBeNull()
})

it('the Advisor tab has the model, the dissatisfied count and groups but no reasoning effort', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openEditDialog('auto-gpt')
  await openTab(/^Advisor/)
  expect(screen.getByRole('combobox', { name: 'Advisor model' })).toHaveValue(
    'gpt-advisor'
  )
  expect(
    within(dialog).getByLabelText(
      'Dissatisfied messages before the advisor answers'
    )
  ).toHaveValue(2)
  expect(within(dialog).getByText('Group order')).toBeVisible()
  expect(within(dialog).queryByText('Reasoning effort')).toBeNull()
  expect(screen.queryByRole('button', { name: 'Fixed' })).toBeNull()
})

it('each tab keeps its group order in saved order', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openEditDialog('auto-gpt')
  const first = screen.getByText('subscription')
  const second = screen.getByText('api')
  expect(
    first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
  await openTab(/^Tiers/)
  await openTab(/^Normal/)
  const normalFirst = screen.getByText('subscription')
  const normalSecond = screen.getByText('api')
  expect(
    normalFirst.compareDocumentPosition(normalSecond) &
      Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
})

it('advanced field hints are behind an info button and not printed in the dialog body', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openEditDialog('auto-gpt')
  await userEvent.click(
    within(dialog).getByRole('button', { name: 'Advanced settings' })
  )
  for (const label of [
    'Downgrade history token limit',
    'Confidence threshold',
    'Simple message length limit (characters)',
  ]) {
    expect(
      within(dialog).getByRole('button', { name: `${label} help` })
    ).toBeInTheDocument()
  }
  for (const hint of [
    'No downgrade once the history exceeds this length, to avoid re-reading the whole history at full price',
    'The default tier is used when the scorer is less sure than this',
    'A latest message longer than this is not treated as simple',
  ]) {
    expect(screen.queryByText(hint)).toBeNull()
  }
})

it('Advanced settings sits below the tab content and above the footer buttons', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openEditDialog('auto-gpt')
  const toggle = within(dialog).getByRole('button', {
    name: 'Advanced settings',
  })
  const scorer = screen.getByRole('combobox', { name: 'Scorer model' })
  const save = within(dialog).getByRole('button', { name: 'Save' })
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(
    scorer.compareDocumentPosition(toggle) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
  expect(
    toggle.compareDocumentPosition(save) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
})

it('the arrow keys move focus between the left tabs', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openEditDialog('auto-gpt')
  const scoring = screen.getByRole('tab', { name: /^Scoring/ })
  await userEvent.click(scoring)
  expect(scoring).toHaveFocus()
  await userEvent.keyboard('{ArrowDown}')
  expect(screen.getByRole('tab', { name: /^Tiers/ })).toHaveFocus()
  await userEvent.keyboard('{ArrowDown}')
  expect(screen.getByRole('tab', { name: /^Advisor/ })).toHaveFocus()
  await userEvent.keyboard('{ArrowUp}')
  expect(screen.getByRole('tab', { name: /^Tiers/ })).toHaveFocus()
})

it('interfaces can be pressed together and the saved route lists every pressed one', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openEditDialog('auto-gpt')
  await userEvent.click(within(dialog).getByRole('button', { name: 'Chat' }))
  await userEvent.click(
    within(dialog).getByRole('button', { name: 'Anthropic' })
  )
  expect(within(dialog).getByRole('button', { name: 'Chat' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  expect(
    within(dialog).getByRole('button', { name: 'Responses' })
  ).toHaveAttribute('aria-pressed', 'true')
  await saveDialog()
  await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1))
  expect([...sentRoute().endpoints].sort()).toEqual([
    'openai',
    'openai-response',
  ])
})

it('saving a complete route with no interface pressed shows the error under the interface buttons and no left tab turns red', async () => {
  const post = mockDialogApis({ errors: [], endpoints: [] })
  show()
  const dialog = await openEditDialog('auto-chat')
  await userEvent.click(within(dialog).getByRole('button', { name: 'Chat' }))
  await saveDialog()
  expect(
    await within(dialog).findByText('Select at least one endpoint')
  ).toBeVisible()
  expect(post).not.toHaveBeenCalled()
  expectSelectedTab(/^Scoring/)
  for (const tab of screen.getAllByRole('tab')) {
    expect(tab).not.toHaveTextContent(/error/i)
  }
})

it('saving a new route with only scoring filled jumps to the first tab with a local error and counts errors on the other tabs', async () => {
  const post = mockDialogApis({ errors: [], endpoints: [] })
  show()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Add virtual model' })
  )
  const dialog = await screen.findByRole('dialog')
  await userEvent.type(within(dialog).getByLabelText(/^Name/), 'auto-new')
  await pickModel('Scorer model', 'gpt-scorer')
  await addGroup('api')
  await saveDialog()
  await waitFor(() => expectSelectedTab(/^Tiers/))
  expect(post).not.toHaveBeenCalled()
  expect(screen.getByRole('tab', { name: /^Tiers/ })).toHaveTextContent(
    /\d+ errors?/
  )
  expect(screen.getByRole('tab', { name: /^Advisor/ })).toHaveTextContent(
    /\d+ errors?/
  )
  expect(screen.getByRole('tab', { name: /^Scoring/ })).not.toHaveTextContent(
    /error/i
  )
  expectSelectedTab(/^Simple/)
  for (const tier of [/^Simple/, /^Normal/, /^Hard/]) {
    expect(screen.getByRole('tab', { name: tier })).toHaveTextContent(/error/i)
  }
})

it('a name error from the server shows under the name and no left tab turns red', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({
    errors: [
      {
        field: 'name',
        message: {
          key: '{{name}} is a real model name and cannot be used by a virtual model',
          params: { name: 'gpt-mid' },
        },
      },
    ],
    endpoints: [],
  })
  show()
  const dialog = await openEditDialog('auto-gpt')
  await saveDialog()
  expect(
    await within(dialog).findByText(
      'gpt-mid is a real model name and cannot be used by a virtual model'
    )
  ).toBeVisible()
  expectSelectedTab(/^Scoring/)
  for (const tab of screen.getAllByRole('tab')) {
    expect(tab).not.toHaveTextContent(/error/i)
  }
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

it('server errors on scoring and a tier group put the dialog on Scoring with red markers on the right tabs and rows', async () => {
  setRoutes([dialogRoute, autoChat])
  const post = mockDialogApis({
    errors: [
      validationError('scorer.model', 'gpt-scorer'),
      {
        field: 'tiers.normal.groups',
        message: {
          key: '{{model}} only has Codex channels in group {{group}} and cannot accept the Anthropic format',
          params: { model: 'gpt-mid', group: 'subscription' },
        },
      },
    ],
    endpoints: [],
  })
  show()
  const dialog = await openEditDialog('auto-gpt')
  await saveDialog()
  expect(
    await within(dialog).findByText(
      'gpt-scorer has no available channel in group api'
    )
  ).toBeVisible()
  expect(post).toHaveBeenCalledTimes(1)
  expect(post.mock.calls[0][0]).toBe('/api/option/smart_routing/validate')
  expect(post.mock.calls[0][1]).toMatchObject({ route: { name: 'auto-gpt' } })
  expectSelectedTab(/^Scoring/)
  expect(screen.getByRole('tab', { name: /^Scoring/ })).toHaveTextContent(
    /1 error/
  )
  expect(screen.getByRole('tab', { name: /^Tiers/ })).toHaveTextContent(
    /1 error/
  )
  expect(screen.getByRole('tab', { name: /^Advisor/ })).not.toHaveTextContent(
    /error/
  )

  await openTab(/^Tiers/)
  expect(screen.getByRole('tab', { name: /^Normal/ })).toHaveTextContent(
    /error/i
  )
  expect(screen.getByRole('tab', { name: /^Simple/ })).not.toHaveTextContent(
    /error/i
  )
  expect(screen.getByRole('tab', { name: /^Hard/ })).not.toHaveTextContent(
    /error/i
  )
  await openTab(/^Normal/)
  expect(
    within(dialog).getByText(
      'gpt-mid only has Codex channels in group subscription and cannot accept the Anthropic format'
    )
  ).toBeVisible()
  expect(screen.getByRole('dialog')).toBeVisible()
  expect(api.patch).not.toHaveBeenCalled()
})

it('the dialog jumps to the first tier with an error, checking Simple then Normal then Hard', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({
    errors: [
      validationError('advisor.model', 'gpt-advisor'),
      validationError('tiers.hard.model', 'gpt-max'),
      validationError('tiers.normal.model', 'gpt-mid'),
    ],
    endpoints: [],
  })
  show()
  const dialog = await openEditDialog('auto-gpt')
  await saveDialog()
  expect(
    await within(dialog).findByText(
      'gpt-mid has no available channel in group api'
    )
  ).toBeVisible()
  expectSelectedTab(/^Tiers/)
  expectSelectedTab(/^Normal/)
  expect(screen.getByRole('tab', { name: /^Hard/ })).toHaveTextContent(/error/i)
  expect(screen.getByRole('tab', { name: /^Advisor/ })).toHaveTextContent(
    /1 error/
  )
})

it('an error only on the advisor puts the dialog on Advisor', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({
    errors: [validationError('advisor.model', 'gpt-advisor')],
    endpoints: [],
  })
  show()
  const dialog = await openEditDialog('auto-gpt')
  await saveDialog()
  expect(
    await within(dialog).findByText(
      'gpt-advisor has no available channel in group api'
    )
  ).toBeVisible()
  expectSelectedTab(/^Advisor/)
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
    route: { name: 'auto-gpt-v2' },
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

async function renameInDialog(from: string, to: string) {
  const dialog = await openEditDialog(from)
  const name = within(dialog).getByLabelText(/^Name/)
  await userEvent.clear(name)
  await userEvent.type(name, to)
  await saveDialog()
  return dialog
}

it('two draft rows can swap names through a temporary one', async () => {
  setRoutes([dialogRoute, autoChat])
  const post = mockDialogApis({ errors: [], endpoints: ['openai-response'] })
  show()

  for (const [from, to] of [
    ['auto-gpt', 'auto-tmp'],
    ['auto-chat', 'auto-gpt'],
    ['auto-tmp', 'auto-chat'],
  ]) {
    await renameInDialog(from, to)
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
  }

  expect(post).toHaveBeenCalledTimes(3)
  expect(screen.getByRole('row', { name: /auto-gpt/ })).toBeVisible()
  expect(screen.getByRole('row', { name: /auto-chat/ })).toBeVisible()
  expect(screen.queryByRole('row', { name: /auto-tmp/ })).toBeNull()
})

it('a deleted saved name can be used again by a new virtual model', async () => {
  const post = mockDialogApis({ errors: [], endpoints: ['openai-response'] })
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

  await userEvent.click(
    screen.getAllByRole('button', { name: 'Add virtual model' })[0]
  )
  await fillNewRoute('auto-gpt')
  await saveDialog()

  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(post).toHaveBeenCalledTimes(1)
  expect(screen.getByRole('row', { name: /auto-gpt/ })).toBeVisible()
})

it('a name changed in the JSON view can be changed back in the visual view', async () => {
  setRoutes([dialogRoute, autoChat])
  const post = mockDialogApis({ errors: [], endpoints: ['openai-response'] })
  show()
  await renameInDialog('auto-gpt', 'auto-x')
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  await writeJson(
    JSON.stringify([{ ...dialogRoute, name: 'auto-y' }, autoChat])
  )
  expect(await screen.findByRole('row', { name: /auto-y/ })).toBeVisible()

  await renameInDialog('auto-y', 'auto-gpt')

  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(post).toHaveBeenCalledTimes(2)
  expect(screen.getByRole('row', { name: /auto-gpt/ })).toBeVisible()
})

it('renaming a draft row onto another draft row fails locally and keeps the dialog open', async () => {
  setRoutes([dialogRoute, autoChat])
  const post = mockDialogApis({ errors: [], endpoints: ['openai-response'] })
  show()

  const dialog = await renameInDialog('auto-chat', 'auto-gpt')

  expect(
    await within(dialog).findByText(
      'A virtual model named auto-gpt already exists'
    )
  ).toBeVisible()
  expect(post).not.toHaveBeenCalled()
  expect(screen.getByRole('dialog')).toBeVisible()
})

it('a new virtual model passes the check and joins the table', async () => {
  const post = mockDialogApis({ errors: [], endpoints: ['openai-response'] })
  show()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Add virtual model' })
  )
  await fillNewRoute('auto-new')
  await saveDialog()
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(post.mock.calls[0][1]).toMatchObject({
    route: {
      name: 'auto-new',
      default_tier: 'normal',
      endpoints: ['openai-response'],
    },
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

it('default tier and follow-scoring range server errors mark their fields', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({
    errors: [
      {
        field: 'tiers.hard.effort_max',
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
  expectSelectedTab(/^Tiers/)
  expectSelectedTab(/^Hard/)
  expect(within(dialog).getByText('Select a default tier')).toBeVisible()
  expect(
    screen.getByRole('combobox', { name: 'Maximum effort' })
  ).toHaveAttribute('aria-invalid', 'true')
  expect(
    screen.getByRole('combobox', { name: 'Default tier' })
  ).toHaveAttribute('aria-invalid', 'true')
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

it('a model field error leaves the model input showing its value with no open list', async () => {
  setRoutes([dialogRoute, autoChat])
  mockDialogApis({
    errors: [validationError('tiers.hard.model', 'gpt-max')],
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
  expectSelectedTab(/^Tiers/)
  expectSelectedTab(/^Hard/)
  expect(screen.getByRole('combobox', { name: 'Hard tier model' })).toHaveValue(
    'gpt-max'
  )
  expect(screen.queryByRole('listbox')).toBeNull()
})

it('the left tab list is vertical on a wide screen and horizontal on a phone', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openEditDialog('auto-gpt')
  expect(screen.getByRole('tablist')).toHaveAttribute(
    'aria-orientation',
    'vertical'
  )
  cleanup()
  const original = window.matchMedia
  vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
    ...original(query),
    matches: query.includes('max-width'),
  }))
  show()
  await openEditDialog('auto-gpt')
  expect(screen.getByRole('tablist')).toHaveAttribute(
    'aria-orientation',
    'horizontal'
  )
})

it('hovering a tier tab shows its model and reasoning effort summary', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  show()
  await openEditDialog('auto-gpt')
  await openTab(/^Tiers/)
  await userEvent.hover(screen.getByRole('tab', { name: /^Hard/ }))
  expect(
    (await screen.findAllByText('gpt-max · Follow scoring medium～xhigh'))
      .length
  ).toBeGreaterThan(0)
})

it('the table shows No thinking on the gpt-4.1 tier of auto-chat', async () => {
  show()
  const chat = await screen.findByRole('row', { name: /auto-chat/ })
  await waitFor(() => expect(chat).toHaveTextContent('No thinking'))
})

it('a route pasted in the JSON view without endpoints keeps the JSON view and shows an incomplete notice', async () => {
  show()
  const { endpoints: _drop, ...withoutEndpoints } = autoGpt
  const editor = await writeJson(JSON.stringify([withoutEndpoints]))
  expect(await screen.findByText(/must include/i)).toBeVisible()
  expect(screen.getByRole('tab', { name: 'JSON' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  expect(editor).toBeInTheDocument()
  expect(screen.getByText('Virtual models')).toBeVisible()
})

it('a failing thinking capability request disables the effort controls and says so', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  const base = vi.mocked(api.get).getMockImplementation()
  if (!base) throw new Error('api.get is not mocked')
  vi.mocked(api.get).mockImplementation(async (url, config) => {
    if (url === '/api/option/smart_routing/thinking_capabilities') {
      throw new Error('capabilities down')
    }
    return base(url, config)
  })
  show()
  const dialog = await openTierPanel('auto-gpt', /^Normal/)
  expect(
    await within(dialog).findByText('Thinking capability failed to load')
  ).toBeVisible()
  expect(screen.queryAllByRole('combobox', { name: /effort/ })).toEqual([])
  for (const button of screen.queryAllByRole('button', {
    name: /^(Not overridden|Fixed|Follow scoring)$/,
  })) {
    expect(button).toBeDisabled()
  }
  expect(screen.queryByRole('option')).toBeNull()
})

it('a stored tier with only an effort value opens as Fixed with that value and can be saved', async () => {
  const post = mockDialogApis({ errors: [], endpoints: [] })
  show()
  const stored = {
    ...dialogRoute,
    tiers: {
      ...dialogRoute.tiers,
      simple: { model: 'gpt-mini', groups: ['api'], effort: 'high' },
    },
  }
  await writeJson(JSON.stringify([stored]))
  await openTierPanel('auto-gpt', /^Simple/)
  expect(screen.getByRole('button', { name: 'Fixed' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  expect(
    screen.getByRole('combobox', { name: 'Reasoning effort' })
  ).toHaveTextContent('high')
  await saveDialog()
  await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
  expect(sentRoute().tiers.simple).toMatchObject({
    effort_mode: 'fixed',
    effort: 'high',
  })
})

it('the Chinese text for a missing interface is the agreed sentence', () => {
  const original = i18next.getResourceBundle('zh', 'translation')
  i18next.addResourceBundle('zh', 'translation', zh.translation, true, true)
  try {
    expect(i18next.getFixedT('zh')('Select at least one endpoint')).toBe(
      '至少选择一种接口'
    )
  } finally {
    i18next.removeResourceBundle('zh', 'translation')
    if (original) i18next.addResourceBundle('zh', 'translation', original)
  }
})

it('the error count key has every plural form in all seven locales, with no Cyrillic outside Russian', () => {
  const locales = { en, fr, ja, ru, vi: viLocale, zh, 'zh-TW': zhTW }
  for (const [name, locale] of Object.entries(locales)) {
    const entries = locale.translation as Record<string, string>
    for (const suffix of ['one', 'few', 'many', 'other']) {
      const value = entries[`{{count}} errors_${suffix}`]
      expect(value, `${name} ${suffix}`).toEqual(expect.any(String))
      if (name !== 'ru') {
        expect(value, `${name} ${suffix}`).not.toMatch(/[\u0400-\u04FF]/)
      }
      if (name === 'zh') expect(value).toBe('{{count}} 个错误')
    }
  }
})

it('the table shows an old-format tier that only has an effort as Fixed with that effort', async () => {
  setRoutes([
    {
      ...autoGpt,
      tiers: {
        ...autoGpt.tiers,
        simple: { model: 'gpt-mini', groups: ['api'], effort: 'high' },
      },
    },
  ])
  show()
  const row = await screen.findByRole('row', { name: /auto-gpt/ })
  expect(row).toHaveTextContent('Fixed high')
})

it('while the capability request is pending the effort mode buttons are disabled and no dropdown shows, then it recovers', async () => {
  mockDialogApis({ errors: [], endpoints: [] })
  const base = vi.mocked(api.get).getMockImplementation()
  if (!base) throw new Error('api.get is not mocked')
  let release: () => void = () => {}
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  vi.mocked(api.get).mockImplementation(async (url, config) => {
    if (url === '/api/option/smart_routing/thinking_capabilities') await gate
    return base(url, config)
  })
  show()
  await openTierPanel('auto-gpt', /^Normal/)
  for (const name of ['Not overridden', 'Fixed', 'Follow scoring']) {
    expect(screen.getByRole('button', { name })).toBeDisabled()
  }
  expect(screen.queryAllByRole('combobox', { name: /effort/ })).toEqual([])
  release()
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Fixed' })).toBeEnabled()
  )
  expect(
    screen.getByRole('combobox', { name: 'Reasoning effort' })
  ).toHaveTextContent('medium')
})
