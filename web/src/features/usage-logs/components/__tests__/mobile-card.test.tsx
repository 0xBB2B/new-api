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
  type VisibilityState,
} from '@tanstack/react-table'
import { render, screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'

import { usageLogSchema, type UsageLog } from '../../data/schema'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { UsageLogsMobileList } from '../usage-logs-mobile-card'
import { UsageLogsProvider, useUsageLogsContext } from '../usage-logs-provider'

const longName = 'enterprise-production-failover-2026-without-any-short-alias'
const log = usageLogSchema.parse({
  id: 1,
  user_id: 2,
  created_at: 1788840000,
  type: 2,
  content: '',
  model_name: longName,
  username: 'production-admin',
  channel: 372,
  channel_name: longName,
  token_name: 'backend-production-token',
  group: 'enterprise-production',
  quota: 123456789,
  use_time: 1.5,
  prompt_tokens: 1200,
  completion_tokens: 800,
  other: JSON.stringify({
    cache_tokens: 300,
    cache_creation_tokens_5m: 200,
    model_ratio: 1,
  }),
})

const displayNameLog = usageLogSchema.parse({
  ...log,
  id: 2,
  user_id: 7,
  username: 'zhangsan',
  display_name: '张三',
})

const idOnlyLog = usageLogSchema.parse({
  ...log,
  id: 3,
  user_id: 8,
  username: '',
  display_name: '',
})

const noUserLog = usageLogSchema.parse({
  ...log,
  id: 4,
  user_id: 0,
  username: '',
  display_name: '',
})

function Fixture(props: {
  admin?: boolean
  visibility?: VisibilityState
  logs?: UsageLog[]
  loading?: boolean
}) {
  const columns = useCommonLogsColumns(props.admin ?? true, false)
  const context = useUsageLogsContext()
  const table = useReactTable({
    data: props.logs ?? [log],
    columns,
    getCoreRowModel: getCoreRowModel(),
    state: { columnVisibility: props.visibility ?? {} },
  })
  return (
    <>
      <button type='button' onClick={() => context.setSensitiveVisible(false)}>
        Hide sensitive data
      </button>
      <UsageLogsMobileList
        table={table}
        logCategory='common'
        isLoading={props.loading}
      />
    </>
  )
}

function renderLogs(props: Parameters<typeof Fixture>[0] = {}) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <UsageLogsProvider>
        <Fixture {...props} />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
}

it('opens long channel text on tap and copies the complete value', async () => {
  const user = userEvent.setup()
  const copy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue()
  renderLogs()
  await user.click(
    screen.getByRole('button', { name: `Channel: ${longName} #372` })
  )
  const dialog = await screen.findByRole('dialog', { name: 'Channel' })
  expect(within(dialog).getByText(`${longName} #372`)).toHaveClass(
    '[overflow-wrap:anywhere]'
  )
  await user.click(
    within(dialog).getByRole('button', { name: 'Copy to clipboard' })
  )
  expect(copy).toHaveBeenCalledWith(`${longName} #372`)
  await user.keyboard('{Escape}')
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(
    screen.getByRole('button', { name: `Channel: ${longName} #372` })
  ).toHaveFocus()
})

it('keeps the model clamped to two lines and exposes its full value with keyboard input', async () => {
  const user = userEvent.setup()
  renderLogs()
  const button = screen.getByRole('button', { name: `Model: ${longName}` })
  expect(within(button).getByText(longName)).toHaveClass(
    'line-clamp-2',
    '[overflow-wrap:anywhere]'
  )
  button.focus()
  await user.keyboard('{Enter}')
  expect(
    await screen.findByRole('dialog', { name: 'Model' })
  ).toHaveTextContent(longName)
})

it('hides sensitive names and disables full-text inspection when privacy is enabled', async () => {
  const user = userEvent.setup()
  renderLogs()
  await user.click(screen.getByRole('button', { name: 'Hide sensitive data' }))
  expect(screen.queryByText('production-admin')).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: /Channel:/ })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: /Token:/ })
  ).not.toBeInTheDocument()
})

it('respects hidden columns and omits admin fields in the self view', () => {
  renderLogs({
    admin: false,
    visibility: { token_name: false, model_name: false },
  })
  expect(
    screen.queryByRole('button', {
      name: /Channel:|User:|Token:|Group:|Model:/,
    })
  ).not.toBeInTheDocument()
  expect(screen.queryByText('enterprise-production')).not.toBeInTheDocument()
})

it('keeps input, output and cache quantities readable without empty metric cells', () => {
  renderLogs()
  expect(screen.getByText('Input')).toBeVisible()
  expect(screen.getByText('Output')).toBeVisible()
  expect(screen.getByText(/300/)).toBeVisible()
  expect(screen.getByText('Cache ↑ 200')).toBeVisible()
})

it('shows the established empty state when no logs exist', () => {
  renderLogs({ logs: [] })
  expect(screen.getByText('No Logs Found')).toBeVisible()
})

it.each([false, true])(
  'keeps timing in the right column for streaming=%s',
  (streaming) => {
    renderLogs({
      logs: [
        {
          ...log,
          is_stream: streaming,
          use_time: 58,
          other: JSON.stringify({ frt: 58000 }),
        },
      ],
    })
    const row = screen
      .getByRole('button', { name: /^Time:/ })
      .closest('[data-slot="log-time-and-timing"]')
    expect(row).toHaveClass('grid', 'grid-cols-2')
    expect(
      screen.getByRole('button', { name: /^Time:/ }).parentElement
    ).toHaveClass('flex-col', 'justify-between')
    expect(
      within(row as HTMLElement)
        .getByText('Duration')
        .closest('.col-start-2')
    ).not.toBeNull()
    if (streaming) {
      expect(within(row as HTMLElement).getByText('First token')).toBeVisible()
    }
  }
)

it('retains the user avatar and model badge in the mobile summary', () => {
  renderLogs()
  expect(screen.getByText('P')).toBeVisible()
  const modelButton = screen.getByRole('button', { name: `Model: ${longName}` })
  expect(modelButton.querySelector('[data-slot="status-badge"]')).not.toBeNull()
})

it('omits unused token and throughput placeholders for async jobs', () => {
  renderLogs({
    logs: [
      {
        ...log,
        prompt_tokens: 0,
        completion_tokens: 0,
        other: JSON.stringify({ is_task: true }),
      },
    ],
  })
  expect(screen.getByText('Async')).toBeVisible()
  expect(screen.queryByText('Input')).not.toBeInTheDocument()
  const timing = screen
    .getByRole('button', { name: /^Time:/ })
    .closest('[data-slot="log-time-and-timing"]')
  expect(within(timing as HTMLElement).queryByText('—')).not.toBeInTheDocument()
})

it('shows mapped model names in full when inspecting a mobile model badge', async () => {
  const user = userEvent.setup()
  renderLogs({
    logs: [
      {
        ...log,
        other: JSON.stringify({
          is_model_mapped: true,
          upstream_model_name:
            'provider-production-mapped-model-with-a-long-name',
        }),
      },
    ],
  })
  await user.click(screen.getByRole('button', { name: `Model: ${longName}` }))
  const dialog = await screen.findByRole('dialog', { name: 'Model' })
  expect(within(dialog).getByText('Actual Model')).toBeVisible()
  expect(
    within(dialog).getByText(
      'provider-production-mapped-model-with-a-long-name'
    )
  ).toBeVisible()
})

it('shows loading placeholders without displaying stale log fields', () => {
  renderLogs({ loading: true })
  expect(screen.getByRole('status', { name: 'Loading' })).toHaveAttribute(
    'aria-busy',
    'true'
  )
  expect(
    screen.queryByRole('button', { name: /^Model:/ })
  ).not.toBeInTheDocument()
})

it('defaults display_name to an empty string and preserves an explicit value', () => {
  const minimal = { id: 10, user_id: 1, created_at: 0, type: 2, content: '' }
  expect(usageLogSchema.parse(minimal).display_name).toBe('')
  expect(
    usageLogSchema.parse({ ...minimal, display_name: '张三' }).display_name
  ).toBe('张三')
})

it('shows the resolved display name in the mobile user field', () => {
  renderLogs({ logs: [displayNameLog] })
  expect(screen.getByRole('button', { name: 'User: 张三' })).toBeVisible()
  expect(screen.queryByText('zhangsan')).not.toBeInTheDocument()
})

it('renders the id fallback name in the mobile user field when no username exists', () => {
  renderLogs({ logs: [idOnlyLog] })
  expect(screen.getByRole('button', { name: 'User: User 8' })).toBeVisible()
})

function DesktopUserCellFixture(props: { logs: UsageLog[] }) {
  const columns = useCommonLogsColumns(true, false)
  const context = useUsageLogsContext()
  const table = useReactTable({
    data: props.logs,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  const userCell = table
    .getRowModel()
    .rows[0]?.getVisibleCells()
    .find((cell) => cell.column.id === 'user')
  return (
    <>
      <button type='button' onClick={() => context.setSensitiveVisible(false)}>
        Hide sensitive data
      </button>
      <span data-testid='selected-user-id'>{String(context.selectedUserId)}</span>
      <span data-testid='dialog-open'>{String(context.userInfoDialogOpen)}</span>
      <div data-testid='user-cell-slot'>
        {userCell
          ? flexRender(userCell.column.columnDef.cell, userCell.getContext())
          : null}
      </div>
    </>
  )
}

function renderUserCell(logs: UsageLog[]) {
  return render(
    <TooltipProvider delay={0}>
      <UsageLogsProvider>
        <DesktopUserCellFixture logs={logs} />
      </UsageLogsProvider>
    </TooltipProvider>
  )
}

it('shows the resolved display name as the user column primary text without leaking the username', async () => {
  const user = userEvent.setup()
  renderUserCell([displayNameLog])
  const slot = screen.getByTestId('user-cell-slot')
  expect(within(slot).getByText('张三')).toBeVisible()
  expect(within(slot).queryByText('zhangsan')).not.toBeInTheDocument()
  await user.hover(within(slot).getByText('张三'))
  expect(await screen.findByText('Username: zhangsan')).toBeVisible()
  expect(await screen.findByText('User ID: 7')).toBeVisible()
})

it('falls back to the localized id name for the user column when username and display_name are empty', () => {
  renderUserCell([idOnlyLog])
  expect(
    within(screen.getByTestId('user-cell-slot')).getByText('User 8')
  ).toBeVisible()
})

it('keeps the user column empty when the row has no user id', () => {
  renderUserCell([noUserLog])
  expect(screen.getByTestId('user-cell-slot')).toBeEmptyDOMElement()
})

it('masks the user column and exposes no identity details while sensitive data is hidden', async () => {
  const user = userEvent.setup()
  renderUserCell([displayNameLog])
  await user.click(screen.getByRole('button', { name: 'Hide sensitive data' }))
  const slot = screen.getByTestId('user-cell-slot')
  expect(within(slot).getByText('••••')).toBeVisible()
  await user.hover(within(slot).getByText('••••'))
  await waitFor(() => {
    expect(screen.queryByText('User ID: 7')).not.toBeInTheDocument()
  })
  expect(document.body.textContent).not.toContain('zhangsan')
})

it('opens the user info dialog with the row user id when the user column is clicked', async () => {
  const user = userEvent.setup()
  renderUserCell([displayNameLog])
  await user.click(
    within(screen.getByTestId('user-cell-slot')).getByRole('button')
  )
  expect(screen.getByTestId('selected-user-id')).toHaveTextContent('7')
  expect(screen.getByTestId('dialog-open')).toHaveTextContent('true')
})
