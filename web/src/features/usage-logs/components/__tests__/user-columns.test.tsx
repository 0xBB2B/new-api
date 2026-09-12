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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'

import type { TaskLog } from '../../types'
import { useTaskLogsColumns } from '../columns/task-logs-columns'
import { TaskDetailsDialog } from '../dialogs/task-details-dialog'
import { UsageLogsProvider, useUsageLogsContext } from '../usage-logs-provider'

const taskA: TaskLog = {
  id: 1,
  user_id: 7,
  username: 'zhangsan',
  display_name: '张三',
  platform: 'suno',
  task_id: 'task-1',
  action: 'MUSIC',
  channel_id: 5,
  group: '',
  quota: 100,
  submit_time: 1788840000,
  status: 'SUCCESS',
}

const taskB: TaskLog = {
  id: 2,
  user_id: 8,
  username: undefined,
  platform: 'kling',
  task_id: 'task-2',
  action: 'GENERATE',
  channel_id: 5,
  group: '',
  quota: 100,
  submit_time: 1788840000,
  status: 'SUCCESS',
}

function UserColumnFixture(props: { logs: TaskLog[] }) {
  const columns = useTaskLogsColumns(true, false)
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
      <span data-testid='selected-user-id'>
        {String(context.selectedUserId)}
      </span>
      <span data-testid='dialog-open'>
        {String(context.userInfoDialogOpen)}
      </span>
      <button type='button' onClick={() => context.setSensitiveVisible(false)}>
        hide sensitive
      </button>
      <div data-testid='user-cell-slot'>
        {userCell
          ? flexRender(userCell.column.columnDef.cell, userCell.getContext())
          : null}
      </div>
    </>
  )
}

function renderUserColumn(logs: TaskLog[]) {
  return render(
    <TooltipProvider delay={0}>
      <UsageLogsProvider>
        <UserColumnFixture logs={logs} />
      </UsageLogsProvider>
    </TooltipProvider>
  )
}

it('masks the task user column and exposes no identity details while sensitive data is hidden', async () => {
  const user = userEvent.setup()
  renderUserColumn([taskA])
  await user.click(screen.getByRole('button', { name: 'hide sensitive' }))
  const slot = screen.getByTestId('user-cell-slot')
  expect(within(slot).getByText('••••')).toBeVisible()
  await user.hover(within(slot).getByText('••••'))
  await waitFor(() => {
    expect(screen.queryByText('User ID: 7')).not.toBeInTheDocument()
  })
  expect(document.body.textContent).not.toContain('zhangsan')
  expect(document.body.textContent).not.toContain('张三')
})

it('shows the resolved display name as the task user column primary text without leaking the username', async () => {
  const user = userEvent.setup()
  renderUserColumn([taskA])
  const slot = screen.getByTestId('user-cell-slot')
  expect(within(slot).getByText('张三')).toBeVisible()
  expect(within(slot).queryByText('zhangsan')).not.toBeInTheDocument()
  await user.hover(within(slot).getByText('张三'))
  expect(await screen.findByText('Username: zhangsan')).toBeVisible()
  expect(await screen.findByText('User ID: 7')).toBeVisible()
})

it('falls back to the localized id name for the task user column when no username exists', () => {
  renderUserColumn([taskB])
  expect(
    within(screen.getByTestId('user-cell-slot')).getByText('User 8')
  ).toBeVisible()
})

it('opens the user info dialog with the row user id when the task user column is clicked', async () => {
  const user = userEvent.setup()
  renderUserColumn([taskA])
  await user.click(
    within(screen.getByTestId('user-cell-slot')).getByRole('button')
  )
  expect(screen.getByTestId('selected-user-id')).toHaveTextContent('7')
  expect(screen.getByTestId('dialog-open')).toHaveTextContent('true')
})

it('shows the resolved display name for the User row in the task details dialog', async () => {
  const user = userEvent.setup()
  render(
    <TooltipProvider delay={0}>
      <TaskDetailsDialog
        log={taskA}
        isAdmin
        isRoot={false}
        open
        onOpenChange={() => {}}
      />
    </TooltipProvider>
  )
  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByText('张三')).toBeVisible()
  expect(within(dialog).queryByText('zhangsan')).not.toBeInTheDocument()
  await user.hover(within(dialog).getByText('张三'))
  expect(await screen.findByText('Username: zhangsan')).toBeVisible()
  expect(await screen.findByText('User ID: 7')).toBeVisible()
})
