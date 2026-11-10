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
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { toast } from 'sonner'
import { afterAll, afterEach, beforeEach, expect, test, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import zh from '@/i18n/locales/zh.json'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import * as api from '../../api'
import type { UsageLog } from '../../data/schema'
import type { LogOtherData } from '../../types'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { DetailsDialog } from '../dialogs/details-dialog'

vi.mock('@lobehub/icons', () => ({}))
vi.hoisted(() => {
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => undefined,
    removeItem: () => undefined,
  })
})
afterAll(() => vi.unstubAllGlobals())

function makeLog(other: LogOtherData, type = 2): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1,
    type,
    content: '',
    username: 'user',
    display_name: '',
    token_name: 'token',
    model_name: 'wan2.5-i2v-preview',
    quota: 5000,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    is_stream: false,
    channel: 1,
    channel_name: '',
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify(other),
    request_id: 'req-1',
    upstream_request_id: '',
  }
}

function DetailPreview(props: {
  other: LogOtherData
  isAdmin: boolean
  type?: number
  columnId?: string
}) {
  const table = useReactTable({
    data: [makeLog(props.other, props.type)],
    columns: useCommonLogsColumns(props.isAdmin, false),
    getCoreRowModel: getCoreRowModel(),
  })
  const cell = table
    .getRowModel()
    .rows[0].getAllCells()
    .find((item) => item.column.id === (props.columnId ?? 'content'))
  if (!cell) throw new Error('The log must have the requested column')
  return flexRender(cell.column.columnDef.cell, cell.getContext())
}
const plugin = {
  key: 'incho',
  name: 'Incho',
  version: '1.0.1',
  author: { name: 'Plugin maintainer' },
}
const previousConfig = useSystemConfigStore.getState().config
let client: QueryClient
const i18n = createInstance()
beforeEach(async () => {
  await i18n.init({
    lng: 'en',
    resources: { en },
    interpolation: { escapeValue: false },
  })
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(['status'], {}, { updatedAt: Date.now() + 60_000 })
  client.setQueryData(
    ['pricing'],
    { data: [], vendors: [] },
    { updatedAt: Date.now() + 60_000 }
  )
})
afterEach(() => {
  vi.restoreAllMocks()
  client.clear()
  useSystemConfigStore.getState().setConfig(previousConfig)
})
function renderPreview(other: LogOtherData, isAdmin = true, type?: number) {
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <DetailPreview other={other} isAdmin={isAdmin} type={type} />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return screen.getByRole('button', { name: /./ })
}

test('keeps log details open when the parent refreshes with unchanged data', async () => {
  const other = { model_price: 0.25 }
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    </I18nextProvider>
  )
  const { rerender } = render(<DetailPreview other={other} isAdmin />, {
    wrapper,
  })
  fireEvent.click(screen.getByRole('button', { name: 'Per-call · $0.25' }))
  expect(await screen.findByRole('dialog')).toBeVisible()
  rerender(<DetailPreview other={other} isAdmin />)
  expect(screen.getByRole('dialog')).toBeVisible()
})

test.each([
  {
    name: 'fixed expression zero price',
    other: {
      billing_mode: 'tiered_expr',
      billing_unit: 'request' as const,
      fixed_price: 0,
      matched_tier: 'free',
      expr_b64: btoa('tier("free", fixed(0))'),
    },
    expected: 'free · Per-call $0/request',
  },
  {
    name: 'fixed expression trace outside the display grammar',
    other: {
      billing_mode: 'tiered_expr',
      billing_unit: 'request' as const,
      fixed_price: 0.01,
      matched_tier: 'priority',
      expr_b64: btoa(
        'param("fast") == true ? tier("priority", fixed(0.01)) : tier("tokens", p * 2)'
      ),
    },
    expected: 'priority · Per-call $0.01/request',
  },
  {
    name: 'parsed token expression',
    other: {
      billing_mode: 'tiered_expr',
      matched_tier: 'standard',
      expr_b64: btoa('tier("standard", p * 2 + c * 10)'),
    },
    expected: 'standard · $2 / $10/M',
  },
  {
    name: 'per-call',
    other: { model_price: 0.25 },
    expected: 'Per-call · $0.25',
  },
  {
    name: 'standard',
    other: { model_ratio: 1, completion_ratio: 2 },
    expected: 'Standard · $2 / $4/M',
  },
  {
    name: 'zero price fallback',
    other: { model_price: 0, group_ratio: 1 },
    expected: 'Group Ratio 1x',
  },
  { name: 'missing price fallback', other: {}, expected: '—' },
])('$name stays visible without a plugin counter', ({ other, expected }) => {
  const preview = renderPreview({
    ...other,
    admin_info: { task_plugin: plugin },
  })
  expect(preview.textContent).toBe(expected)
})

test.each([
  {
    tier: 'standard',
    billingUnit: 'token' as const,
    fixedPrice: undefined,
    expected: 'Dynamic Pricing · standard',
  },
  {
    tier: 'long_context',
    billingUnit: 'token' as const,
    fixedPrice: undefined,
    expected: 'Dynamic Pricing · long_context',
  },
  {
    tier: 'free_small_output',
    billingUnit: 'request' as const,
    fixedPrice: 0,
    expected: 'free_small_output · Per-call $0/request',
  },
])(
  'nested expression preserves the recorded $tier settlement in the list',
  ({ tier, billingUnit, fixedPrice, expected }) => {
    const expression = `len <= 272000
      ? (c > 5
        ? tier("standard", p * 2 + c * 10 + cr * 0.1 + cc * 2.5)
        : tier("free_small_output", fixed(0)))
      : (c > 5
        ? tier("long_context", p * 5 + c * 22.5 + cr * 0.5)
        : tier("free_small_output", fixed(0)))`
    const preview = renderPreview({
      billing_mode: 'tiered_expr',
      billing_unit: billingUnit,
      fixed_price: fixedPrice,
      matched_tier: tier,
      expr_b64: btoa(expression),
    })
    expect(preview.textContent).toBe(expected)
  }
)

test.each([
  { name: 'missing expression', expr: undefined },
  { name: 'invalid encoded expression', expr: '%' },
  {
    name: 'tier absent from the parsed expression',
    expr: btoa('tier("different", p * 99 + c * 99)'),
  },
])(
  'token pricing with $name preserves the saved tier without guessing prices',
  ({ expr }) => {
    const preview = renderPreview({
      billing_mode: 'tiered_expr',
      matched_tier: 'recorded',
      expr_b64: expr,
    })
    expect(preview.textContent).toBe('Dynamic Pricing · recorded')
  }
)

test.each([false, true])(
  'dynamic pricing without a saved tier uses a localized neutral fallback for task=%s',
  async (isTask) => {
    i18n.addResourceBundle('zh', 'translation', zh.translation)
    const preview = renderPreview({
      is_task: isTask,
      billing_mode: 'tiered_expr',
      expr_b64: btoa(
        isTask
          ? 'tier("base", u("clips") * 0.25)'
          : 'tier("base", p * 2 + c * 10)'
      ),
    })
    expect(preview.textContent).toBe(
      'Dynamic Pricing · Special billing expression'
    )
    await act(() => i18n.changeLanguage('zh'))
    expect(
      screen.getByRole('button', { name: '动态计费 · 特殊计费表达式' })
    ).toBeVisible()
  }
)

test('top-up content_parts render in the viewer language', async () => {
  i18n.addResourceBundle('zh', 'translation', {
    'Online top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}':
      '在线充值成功，充值额度：{{quota}}，支付金额：{{amount}}',
  })
  await i18n.changeLanguage('zh')
  const preview = renderPreview(
    {
      content_parts: [
        {
          key: 'Online top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}',
          params: { quota: '$10.000000', amount: '10.00' },
        },
      ],
    },
    false,
    1
  )
  expect(preview.textContent).toBe(
    '在线充值成功，充值额度：$10.000000，支付金额：10.00'
  )
})

test('quota saturation remains first and only billing adds to the counter', () => {
  const preview = renderPreview({
    model_price: 0.25,
    admin_info: {
      task_plugin: plugin,
      quota_saturation: {
        op: 'round',
        kind: 'overflow',
        original: 3e9,
        clamped: 2147483647,
      },
    },
  })
  expect(preview.textContent).toBe('Quota clamped+1')
})

test.each([true, false])(
  'plugin information in the opened dialog respects admin=%s',
  async (isAdmin) => {
    const preview = renderPreview(
      { model_price: 0.25, admin_info: { task_plugin: plugin } },
      isAdmin
    )
    expect(preview.textContent).toBe('Per-call · $0.25')
    fireEvent.click(preview)
    const dialog = within(await screen.findByRole('dialog'))
    if (isAdmin) {
      expect(dialog.getByText('Incho')).toBeVisible()
      expect(dialog.getByText('1.0.1')).toBeVisible()
      expect(dialog.getByText('Plugin maintainer')).toBeVisible()
    } else {
      expect(dialog.queryByText('Incho')).not.toBeInTheDocument()
      expect(dialog.queryByText('Plugin maintainer')).not.toBeInTheDocument()
    }
  }
)

test.each([
  {
    expression: 'tier("music", u("clips") * 0.25)',
    tier: 'music',
    expected: 'music · clips $0.25/unit',
  },
  {
    expression:
      'u("mode") == "pro" ? tier("pro", u("seconds") * 0.8) : tier("std", u("seconds") * 0.4)',
    tier: 'pro',
    expected: 'pro · seconds $0.8/second',
  },
  {
    expression: 'tier("tokens", u("tokens") * 9.8 / 1000000)',
    tier: 'tokens',
    expected: 'tokens · tokens $9.8/1M token',
  },
  {
    expression: 'tier("free", u("clips") * 0)',
    tier: 'free',
    expected: 'free · clips $0/unit',
  },
  {
    expression: 'tier("mixed", 0.1 + u("clips") * 0.25 + u("units") * 0.14)',
    tier: 'mixed',
    expected:
      'mixed · clips $0.25/unit · units $0.14/credit · Additional charge $0.1/request',
  },
])(
  'task expression $tier shows its recorded unit price',
  ({ expression, tier, expected }) => {
    client.setQueryData(['pricing'], {
      data: [
        {
          model_name: 'wan2.5-i2v-preview',
          billing_expr: 'tier("current", u("clips") * 99)',
          billing_usage_schema: {
            clips: { type: 'number', unit: 'count' },
            seconds: { type: 'number', unit: 'second' },
            tokens: { type: 'number', unit: 'token' },
            units: { type: 'number', unit: 'credit' },
            mode: { enum: ['pro', 'std'] },
          },
        },
      ],
      vendors: [],
    })
    const preview = renderPreview({
      is_task: true,
      billing_mode: 'tiered_expr',
      expr_b64: Buffer.from(expression).toString('base64'),
      matched_tier: tier,
      model_price: 0,
      admin_info: { task_plugin: plugin },
    })
    expect(preview.textContent).toBe(expected)
  }
)

test('task log prices use localized unit labels from pricing metadata', async () => {
  client.setQueryData(['pricing'], {
    data: [
      {
        model_name: 'wan2.5-i2v-preview',
        billing_usage_schema: {
          images: {
            type: 'number',
            unit: 'count',
            unitLabel: { en: 'image', zh: '张' },
          },
        },
      },
    ],
    vendors: [],
  })
  const preview = renderPreview({
    is_task: true,
    billing_mode: 'tiered_expr',
    expr_b64: btoa('tier("images", u("images") * 0.25)'),
    matched_tier: 'images',
  })
  expect(preview).toHaveTextContent('images · images $0.25/image')
  await act(() => i18n.changeLanguage('zh-CN'))
  expect(
    screen.getByRole('button', { name: /images · images/ })
  ).toHaveTextContent('images · images $0.25/张')
})

test('task log prices select the executing provider’s schema', () => {
  client.setQueryData(['pricing'], {
    data: [
      {
        model_name: 'wan2.5-i2v-preview',
        billing_usage_schema: { seconds: { type: 'number', unit: 'second' } },
        billing_plugin_variants: [
          {
            plugin_key: 'beta',
            plugin_name: 'Beta',
            billing_expr: 'tier("images", u("images") * 0.25)',
            billing_usage_schema: {
              images: {
                type: 'number',
                unit: 'count',
                unitLabel: { en: 'image' },
              },
            },
          },
        ],
      },
    ],
    vendors: [],
  })
  const preview = renderPreview({
    is_task: true,
    billing_mode: 'tiered_expr',
    expr_b64: btoa('tier("images", u("images") * 0.25)'),
    matched_tier: 'images',
    admin_info: {
      task_plugin: { key: 'beta', name: 'Beta', version: '1.0.0' },
    },
  })
  expect(preview).toHaveTextContent('images · images $0.25/image')
})

test.each(['missing schema', 'unsupported expression', 'unknown tier'])(
  'task pricing with %s preserves the saved tier without guessing prices',
  (scenario) => {
    if (scenario !== 'missing schema') {
      client.setQueryData(['pricing'], {
        data: [
          {
            model_name: 'wan2.5-i2v-preview',
            billing_usage_schema: { clips: { type: 'number', unit: 'count' } },
          },
        ],
        vendors: [],
      })
    }
    const expression =
      scenario === 'unsupported expression'
        ? 'tier("music", max(u("clips"), 1) * 0.25)'
        : 'tier("music", u("clips") * 0.25)'
    const preview = renderPreview({
      is_task: true,
      billing_mode: 'tiered_expr',
      expr_b64: Buffer.from(expression).toString('base64'),
      matched_tier: scenario === 'unknown tier' ? 'old' : 'music',
    })
    expect(preview.textContent).toBe(
      `Dynamic Pricing · ${scenario === 'unknown tier' ? 'old' : 'music'}`
    )
  }
)

type RoutedOther = LogOtherData & { smart_routing?: Record<string, unknown> }

const mainRouting = {
  role: 'main',
  virtual_model: 'auto-model',
  tier: 'hard',
  reason: 'low_confidence',
  reason_params: {},
}
const scorerRouting = {
  role: 'scorer',
  virtual_model: 'auto-model',
  main_tier: 'hard',
  main_model: 'main-model-x',
}

const fullLabels = {
  task_type: 'lookup',
  reasoning: 'low',
  workload: 'one_step',
  risk: 'low',
  domain: 'code',
  confidence: 0.91,
  feedback: 'satisfied',
}

function renderModelCell(other: RoutedOther) {
  const { container } = render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <DetailPreview other={other} isAdmin columnId='model_name' />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return container
}

function renderDialog(other: RoutedOther, id = 1, isAdmin = true) {
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <DetailsDialog
          log={{ ...makeLog(other), id }}
          isAdmin={isAdmin}
          isRoot={false}
          open
          onOpenChange={() => undefined}
        />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return within(screen.getByRole('dialog'))
}

test.each([
  {
    name: 'a normal main request',
    routing: { ...mainRouting, tier: 'hard' },
    text: 'auto-model · Hard tier',
    color: 'text-muted-foreground',
  },
  {
    name: 'an advisor request',
    routing: { ...mainRouting, tier: 'advisor', reason: 'advisor' },
    text: 'auto-model · Advisor',
    color: 'text-warning',
  },
  {
    name: 'a failed scoring request',
    routing: {
      ...mainRouting,
      tier: 'normal',
      reason: 'scorer_error',
      scorer: { status: 'error', billed: false },
    },
    text: 'auto-model · Normal tier · Scoring failed',
    color: 'text-destructive',
  },
  {
    name: 'a score record',
    routing: scorerRouting,
    text: 'Routing score · auto-model',
    color: 'text-muted-foreground',
  },
])('model column shows $name routing line', ({ routing, text, color }) => {
  renderModelCell({ smart_routing: routing })
  expect(screen.getByText(text)).toHaveClass(color)
})

test.each(['timeout', 'invalid'])(
  'model column marks scorer status %s as scoring failed',
  (status) => {
    renderModelCell({
      smart_routing: { ...mainRouting, tier: 'normal', scorer: { status } },
    })
    expect(
      screen.getByText('auto-model · Normal tier · Scoring failed')
    ).toHaveClass('text-destructive')
  }
)

test('routing line is translated in zh', async () => {
  i18n.addResourceBundle('zh', 'translation', zh.translation)
  await i18n.changeLanguage('zh')
  renderModelCell({ smart_routing: mainRouting })
  expect(screen.getByText('auto-model · 困难档')).toBeVisible()
})

test('downgrade-blocked main request shows the scored tier and reason', async () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      tier: 'hard',
      scored_tier: 'simple',
      reason: 'downgrade_blocked',
      reason_params: {
        history_tokens: 90000,
        limit: 60000,
        kept_tier: 'hard',
        scored_tier: 'simple',
      },
    },
  })
  expect(dialog.getByText('Smart routing')).toBeVisible()
  expect(dialog.getByText('Virtual model')).toBeVisible()
  expect(dialog.getByText('auto-model')).toBeVisible()
  expect(dialog.getByText('Hard tier (scored: Simple tier)')).toBeVisible()
  expect(
    dialog.getByText(
      'History is 90,000 tokens, over the downgrade limit 60,000. Kept the previous Hard tier'
    )
  ).toBeVisible()
})

test('downgrade-blocked tier row is translated in zh', async () => {
  i18n.addResourceBundle('zh', 'translation', zh.translation)
  await i18n.changeLanguage('zh')
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      tier: 'hard',
      scored_tier: 'simple',
      reason: 'downgrade_blocked',
      reason_params: {
        history_tokens: 90000,
        limit: 60000,
        kept_tier: 'hard',
        scored_tier: 'simple',
      },
    },
  })
  expect(dialog.getByText('困难档（打分结果：简单档）')).toBeVisible()
  expect(
    dialog.getByText(
      '对话历史 90,000 token，超过降档上限 60,000，保留上一轮的困难档'
    )
  ).toBeVisible()
})

test.each([
  {
    name: 'timeout',
    routing: {
      reason: 'scorer_timeout',
      reason_params: { timeout_ms: 3000 },
      scorer: { status: 'timeout', billed: false },
    },
    reason: 'Scoring timed out (3,000 ms). Used the default tier',
  },
  {
    name: 'error',
    routing: {
      reason: 'scorer_error',
      scorer: { status: 'error', billed: false },
    },
    reason: 'Scoring failed. Used the default tier',
  },
])(
  'scoring $name shows the default tier, a red reason and no billing',
  ({ routing, reason }) => {
    const dialog = renderDialog({
      smart_routing: { ...mainRouting, tier: 'normal', ...routing },
    })
    expect(dialog.getByText('Normal tier (default tier)')).toBeVisible()
    expect(dialog.getByText(reason)).toHaveClass('text-destructive')
    expect(dialog.getByText('No usage returned, not billed')).toBeVisible()
    expect(dialog.queryByText('View this score')).not.toBeInTheDocument()
  }
)

test('invalid score result that was billed links to the score record', () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      tier: 'normal',
      reason: 'scorer_invalid',
      scorer: { status: 'invalid', billed: true },
    },
  })
  expect(dialog.getByText('Normal tier (default tier)')).toBeVisible()
  expect(
    dialog.getByText('Invalid score result. Used the default tier')
  ).toHaveClass('text-destructive')
  expect(dialog.getByText('View this score')).toBeVisible()
  expect(
    dialog.queryByText('No usage returned, not billed')
  ).not.toBeInTheDocument()
})

test('downgrade blocked during a scoring timeout keeps the scored tier label', () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      tier: 'hard',
      scored_tier: 'normal',
      default_tier: 'normal',
      reason: 'downgrade_blocked',
      reason_params: {
        history_tokens: 90000,
        limit: 60000,
        kept_tier: 'hard',
        scored_tier: 'normal',
      },
      scorer: { status: 'timeout', billed: false },
    },
  })
  expect(dialog.getByText('Hard tier (scored: Normal tier)')).toBeVisible()
  expect(dialog.queryByText(/default tier\)/)).not.toBeInTheDocument()
})

test.each([
  {
    reason: 'advisor',
    params: { count: 3 },
    tier: 'advisor',
    text: 'The user was dissatisfied 3 messages in a row',
  },
  {
    reason: 'low_confidence',
    params: {},
    tier: 'hard',
    text: 'Scorer confidence is below the threshold',
  },
  {
    reason: 'rule_simple',
    params: {},
    tier: 'simple',
    text: 'Matched the simple tier rule',
  },
  {
    reason: 'rule_normal',
    params: {},
    tier: 'normal',
    text: 'No special rule matched',
  },
])('reason $reason is explained in the tier reason row', (item) => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      tier: item.tier,
      reason: item.reason,
      reason_params: item.params,
    },
  })
  expect(dialog.getByText('Tier reason')).toBeVisible()
  expect(dialog.getByText(item.text)).toBeVisible()
})

test('reasoning content row appears only when something was cleaned', () => {
  const first = renderDialog({
    smart_routing: { ...mainRouting, cleaned_reasoning: 2 },
  })
  expect(first.getByText('Reasoning content')).toBeVisible()
  expect(
    first.getByText('The previous turn used another channel. Cleaned 2 items')
  ).toBeVisible()
})

test('reasoning content row is hidden when nothing was cleaned', () => {
  const dialog = renderDialog({
    smart_routing: { ...mainRouting, cleaned_reasoning: 0 },
  })
  expect(dialog.queryByText('Reasoning content')).not.toBeInTheDocument()
})

test('tool continuation has no score labels or score record rows', () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      reason: 'tool_continuation',
      labels: { task_type: 'analyze' },
    },
  })
  expect(
    dialog.getByText('Tool result continuation. Kept the tier of this turn')
  ).toBeVisible()
  expect(dialog.queryByText('Score labels')).not.toBeInTheDocument()
  expect(dialog.queryByText('Score record')).not.toBeInTheDocument()
})

test('score record dialog shows type, virtual model and owning request', () => {
  const dialog = renderDialog({ smart_routing: scorerRouting }, 2)
  expect(dialog.getByText('Type')).toBeVisible()
  expect(dialog.getByText('Routing score')).toBeVisible()
  expect(dialog.getByText('auto-model')).toHaveClass('font-mono')
  expect(dialog.getByText('Belongs to request')).toBeVisible()
  expect(dialog.getByText('req-1 (Hard tier · main-model-x)')).toBeVisible()
})

test('main request shows the virtual model in mono font', () => {
  const dialog = renderDialog({ smart_routing: mainRouting })
  expect(dialog.getByText('auto-model')).toHaveClass('font-mono')
})

test('owning request label is translated in zh', async () => {
  i18n.addResourceBundle('zh', 'translation', zh.translation)
  await i18n.changeLanguage('zh')
  const dialog = renderDialog({ smart_routing: scorerRouting }, 2)
  expect(dialog.getByText('req-1（困难档 · main-model-x）')).toBeVisible()
})

test('view this score opens the matching score record', async () => {
  const getAllLogs = vi.spyOn(api, 'getAllLogs').mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          ...makeLog({ smart_routing: scorerRouting } as RoutedOther),
          id: 2,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    },
  })
  const dialog = renderDialog({
    smart_routing: { ...mainRouting, scorer: { status: 'ok', billed: true } },
  })
  fireEvent.click(dialog.getByText('View this score'))
  expect(await screen.findByText('Belongs to request')).toBeVisible()
  expect(getAllLogs).toHaveBeenCalledWith(
    expect.objectContaining({
      request_id: 'req-1',
      start_timestamp: 1 - 3600,
      end_timestamp: 1 + 3600,
    })
  )
  expect(
    screen.getAllByRole('dialog', { hidden: true }).length
  ).toBeGreaterThan(1)
})

test('view this score reports when no matching record exists', async () => {
  vi.spyOn(api, 'getAllLogs').mockResolvedValue({
    success: true,
    data: { items: [], total: 0, page: 1, page_size: 20 },
  })
  const notify = [
    vi.spyOn(toast, 'error'),
    vi.spyOn(toast, 'info'),
    vi.spyOn(toast, 'warning'),
  ]
  const dialog = renderDialog({
    smart_routing: { ...mainRouting, scorer: { status: 'ok', billed: true } },
  })
  fireEvent.click(dialog.getByText('View this score'))
  await vi.waitFor(() =>
    expect(
      notify.some((spy) =>
        spy.mock.calls.some((call) => call[0] === 'Matching record not found')
      )
    ).toBe(true)
  )
  expect(screen.getAllByRole('dialog')).toHaveLength(1)
})

const billedMain = {
  smart_routing: { ...mainRouting, scorer: { status: 'ok', billed: true } },
}

function logsResponse(items: UsageLog[]) {
  return {
    success: true,
    data: { items, total: items.length, page: 1, page_size: 20 },
  }
}

test('regular users look up the score record through their own log API', async () => {
  const getAllLogs = vi.spyOn(api, 'getAllLogs')
  const getUserLogs = vi
    .spyOn(api, 'getUserLogs')
    .mockResolvedValue(
      logsResponse([
        { ...makeLog({ smart_routing: scorerRouting } as RoutedOther), id: 1 },
      ]) as Awaited<ReturnType<typeof api.getUserLogs>>
    )
  const dialog = renderDialog(billedMain, 1, false)
  fireEvent.click(dialog.getByText('View this score'))
  expect(await screen.findByText('Belongs to request')).toBeVisible()
  expect(getUserLogs).toHaveBeenCalledWith(
    expect.objectContaining({ request_id: 'req-1' })
  )
  expect(getAllLogs).not.toHaveBeenCalled()
})

test('a failed lookup shows an error and opens nothing', async () => {
  vi.spyOn(api, 'getAllLogs').mockRejectedValue(new Error('network down'))
  const error = vi.spyOn(toast, 'error')
  const dialog = renderDialog(billedMain)
  fireEvent.click(dialog.getByText('View this score'))
  await vi.waitFor(() => expect(error).toHaveBeenCalled())
  expect(screen.getAllByRole('dialog')).toHaveLength(1)
})

test('a rejected lookup response shows the server message', async () => {
  vi.spyOn(api, 'getAllLogs').mockResolvedValue({
    success: false,
    message: 'Time range too large',
  })
  const error = vi.spyOn(toast, 'error')
  const dialog = renderDialog(billedMain)
  fireEvent.click(dialog.getByText('View this score'))
  await vi.waitFor(() =>
    expect(
      error.mock.calls.some((call) =>
        String(call[0]).includes('Time range too large')
      )
    ).toBe(true)
  )
  expect(screen.getAllByRole('dialog')).toHaveLength(1)
})

test('clicking view this score twice queries only once', async () => {
  const getAllLogs = vi
    .spyOn(api, 'getAllLogs')
    .mockReturnValue(new Promise(() => undefined))
  const dialog = renderDialog(billedMain)
  const link = dialog.getByText('View this score')
  fireEvent.click(link)
  fireEvent.click(link)
  await act(() => Promise.resolve())
  expect(getAllLogs).toHaveBeenCalledTimes(1)
})

test('the score record links back to the main request', async () => {
  vi.spyOn(api, 'getAllLogs').mockResolvedValue(
    logsResponse([
      {
        ...makeLog({ smart_routing: mainRouting } as RoutedOther),
        id: 1,
      },
    ])
  )
  const dialog = renderDialog({ smart_routing: scorerRouting }, 2)
  fireEvent.click(dialog.getByRole('button', { name: /^req-1/ }))
  expect(await screen.findByText('Tier reason')).toBeVisible()
})

test('logs without smart_routing show no routing text', () => {
  const container = renderModelCell({ model_price: 0.25 })
  expect(container.textContent).not.toMatch(/ · (Simple|Normal|Hard) tier/)
  expect(container.textContent).not.toContain('Routing score')
})

test('dialog of a log without smart_routing shows no routing rows', () => {
  const dialog = renderDialog({ model_price: 0.25 })
  for (const text of [
    'Smart routing',
    'Virtual model',
    'Tier reason',
    'Score labels',
    'Score record',
    'Belongs to request',
  ]) {
    expect(dialog.queryByText(text)).not.toBeInTheDocument()
  }
})

test('rule_hard reason lists the matched labels with their values', () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      reason: 'rule_hard',
      reason_params: { matched: ['risk', 'reasoning'] },
      labels: { ...fullLabels, risk: 'high', reasoning: 'high' },
    },
  })
  expect(
    dialog.getByText('Risk of error=High, Reasoning depth=High')
  ).toBeVisible()
})

test('score labels row lists every label in order', () => {
  const dialog = renderDialog({
    smart_routing: { ...mainRouting, labels: fullLabels },
  })
  expect(dialog.getByText('Score labels')).toBeVisible()
  expect(
    dialog.getByText(
      'Task type: Look up information · Reasoning depth: Low · Workload: One step · Risk of error: Low · Work domain: Code · Confidence: 0.91 · User feedback: Satisfied'
    )
  ).toBeVisible()
})

test('score labels row is translated in zh', async () => {
  i18n.addResourceBundle('zh', 'translation', zh.translation)
  await i18n.changeLanguage('zh')
  const dialog = renderDialog({
    smart_routing: { ...mainRouting, labels: fullLabels },
  })
  expect(
    dialog.getByText(
      '任务类型：查找资料或信息 · 要想多深：低 · 工作量：一步 · 出错代价：低 · 工作类型：代码 · 把握：0.91 · 用户反馈：满意'
    )
  ).toBeVisible()
})

test('score labels confidence follows the French decimal comma with two decimals', async () => {
  i18n.addResourceBundle('fr', 'translation', fr.translation)
  await i18n.changeLanguage('fr')
  const dialog = renderDialog({
    smart_routing: { ...mainRouting, labels: fullLabels },
  })
  expect(dialog.getByText(/0,91/)).toBeVisible()
  expect(dialog.queryByText(/0\.91/)).toBeNull()
})

test('score labels put Thinking need right after Reasoning depth', () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      labels: { ...fullLabels, effort_score: 0.72 },
    },
  })
  expect(
    dialog.getByText(
      'Task type: Look up information · Reasoning depth: Low · Thinking need: 0.72 · Workload: One step · Risk of error: Low · Work domain: Code · Confidence: 0.91 · User feedback: Satisfied'
    )
  ).toBeVisible()
})

test('score labels Thinking need keeps two decimals', () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      labels: { ...fullLabels, effort_score: 0.5 },
    },
  })
  expect(dialog.getByText(/Thinking need: 0\.50 ·/)).toBeVisible()
})

test('score labels show an invalid Thinking need as Invalid', () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      labels: { ...fullLabels, effort_score: 'invalid' },
    },
  })
  expect(
    dialog.getByText(
      'Task type: Look up information · Reasoning depth: Low · Thinking need: Invalid · Workload: One step · Risk of error: Low · Work domain: Code · Confidence: 0.91 · User feedback: Satisfied'
    )
  ).toBeVisible()
})

test('score labels without effort_score show no Thinking need', () => {
  const dialog = renderDialog({
    smart_routing: { ...mainRouting, labels: fullLabels },
  })
  expect(dialog.queryByText(/Thinking need/)).toBeNull()
})

test('score labels Thinking need is translated in zh', async () => {
  i18n.addResourceBundle('zh', 'translation', zh.translation)
  await i18n.changeLanguage('zh')
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      labels: { ...fullLabels, effort_score: 0.72 },
    },
  })
  expect(
    dialog.getByText(
      '任务类型：查找资料或信息 · 要想多深：低 · 思考需求：0.72 · 工作量：一步 · 出错代价：低 · 工作类型：代码 · 把握：0.91 · 用户反馈：满意'
    )
  ).toBeVisible()
})

test('score labels invalid Thinking need is translated in zh', async () => {
  i18n.addResourceBundle('zh', 'translation', zh.translation)
  await i18n.changeLanguage('zh')
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      labels: { ...fullLabels, effort_score: 'invalid' },
    },
  })
  expect(
    dialog.getByText(/要想多深：低 · 思考需求：无效 · 工作量/)
  ).toBeVisible()
})

test.each([
  {
    name: 'follow',
    effort: { value: 'high', source: 'follow' },
    text: 'high (Follow scoring)',
  },
  {
    name: 'fixed',
    effort: { value: 'medium', source: 'fixed' },
    text: 'medium (Fixed)',
  },
  {
    name: 'client',
    effort: { value: 'low', source: 'client' },
    text: 'low (Not overridden, sent by the client)',
  },
  {
    name: 'follow lowered from a higher level',
    effort: { value: 'high', source: 'follow', from: 'xhigh' },
    text: 'high (Follow scoring, lowered from xhigh)',
  },
  {
    name: 'client lowered from a higher level',
    effort: { value: 'high', source: 'client', from: 'xhigh' },
    text: 'high (Not overridden, lowered from xhigh)',
  },
  {
    name: 'client raised from a lower level',
    effort: { value: 'low', source: 'client', from: 'minimal' },
    text: 'low (Not overridden, raised from minimal)',
  },
  {
    name: 'no_thinking',
    effort: { value: '', source: 'no_thinking' },
    text: 'Not sent (model does not support thinking)',
  },
  {
    name: 'client_unset',
    effort: { value: '', source: 'client_unset' },
    text: 'Not sent (client did not specify)',
  },
])('reasoning effort row shows $text for $name', ({ effort, text }) => {
  const dialog = renderDialog({
    smart_routing: { ...mainRouting, effort },
  })
  expect(dialog.getByText('Reasoning effort')).toBeVisible()
  expect(dialog.getByText(text)).toBeVisible()
})

test.each([
  { effort: { value: 'high', source: 'follow' }, text: 'high（跟随打分）' },
  { effort: { value: 'medium', source: 'fixed' }, text: 'medium（固定）' },
  {
    effort: { value: 'low', source: 'client' },
    text: 'low（不覆盖，客户端传入）',
  },
  {
    effort: { value: 'high', source: 'follow', from: 'xhigh' },
    text: 'high（跟随打分，由 xhigh 降档）',
  },
  {
    effort: { value: 'high', source: 'client', from: 'xhigh' },
    text: 'high（不覆盖，由 xhigh 降档）',
  },
  {
    effort: { value: 'low', source: 'client', from: 'minimal' },
    text: 'low（不覆盖，由 minimal 升档）',
  },
  {
    effort: { value: '', source: 'no_thinking' },
    text: '未发送（模型不支持思考）',
  },
  {
    effort: { value: '', source: 'client_unset' },
    text: '未发送（客户端未指定）',
  },
])('reasoning effort row is translated in zh as $text', async ({ effort, text }) => {
  i18n.addResourceBundle('zh', 'translation', zh.translation)
  await i18n.changeLanguage('zh')
  const dialog = renderDialog({ smart_routing: { ...mainRouting, effort } })
  expect(dialog.getByText('思考力度')).toBeVisible()
  expect(dialog.getByText(text)).toBeVisible()
})

test('reasoning effort row sits directly below the tier row', () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      effort: { value: 'high', source: 'follow' },
    },
  })
  const tierRow = dialog.getByText('Tier').parentElement
  expect(tierRow?.nextElementSibling).toBe(
    dialog.getByText('Reasoning effort').parentElement
  )
})

test('tool continuation main request still shows the reasoning effort row', () => {
  const dialog = renderDialog({
    smart_routing: {
      ...mainRouting,
      reason: 'tool_continuation',
      effort: { value: 'high', source: 'follow' },
    },
  })
  expect(dialog.getByText('high (Follow scoring)')).toBeVisible()
})

test('reasoning effort row is the same for a non-admin user', () => {
  const dialog = renderDialog(
    {
      smart_routing: {
        ...mainRouting,
        effort: { value: 'high', source: 'follow' },
      },
    },
    1,
    false
  )
  expect(dialog.getByText('high (Follow scoring)')).toBeVisible()
})

test('score record dialog has no reasoning effort row', () => {
  const dialog = renderDialog(
    {
      smart_routing: {
        ...scorerRouting,
        effort: { value: 'high', source: 'follow' },
      },
    },
    2
  )
  expect(dialog.queryByText('Reasoning effort')).not.toBeInTheDocument()
})

test('old main log without effort shows no reasoning effort row and still renders', () => {
  const dialog = renderDialog({ smart_routing: mainRouting })
  expect(dialog.getByText('Tier')).toBeVisible()
  expect(dialog.queryByText('Reasoning effort')).not.toBeInTheDocument()
})
