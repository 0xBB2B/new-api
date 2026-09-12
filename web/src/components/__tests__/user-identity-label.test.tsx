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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'

import { UserIdentityLabel } from '../user-identity-label'

function renderLabel(props: Parameters<typeof UserIdentityLabel>[0]) {
  return render(
    <TooltipProvider>
      <UserIdentityLabel {...props} />
    </TooltipProvider>
  )
}

describe('UserIdentityLabel', () => {
  test('shows display_name as the primary text and reveals username/user id on hover without leaking username into the primary text', async () => {
    const user = userEvent.setup()
    renderLabel({
      user: { user_id: 7, username: 'zhangsan', display_name: '张三' },
    })
    const primary = screen.getByText('张三')
    expect(primary).not.toHaveTextContent('zhangsan')
    await user.hover(primary)
    expect(await screen.findByText('Username: zhangsan')).toBeVisible()
    expect(await screen.findByText('User ID: 7')).toBeVisible()
  })

  test('shows the localized fallback name and only the user id tooltip line when no username or display_name exists', async () => {
    const user = userEvent.setup()
    renderLabel({ user: { user_id: 7 } })
    await user.hover(screen.getByText('User 7'))
    expect(await screen.findByText('User ID: 7')).toBeVisible()
    expect(screen.queryByText(/^Username:/)).not.toBeInTheDocument()
  })

  test('masks the primary text and never reveals username or user id, even on hover', async () => {
    const user = userEvent.setup()
    renderLabel({
      masked: true,
      user: { user_id: 7, username: 'zhangsan', display_name: '张三' },
    })
    expect(screen.getByText('••••')).toBeInTheDocument()
    await user.hover(screen.getByText('••••'))
    await waitFor(() => {
      expect(screen.queryByText('Username: zhangsan')).not.toBeInTheDocument()
      expect(screen.queryByText('User ID: 7')).not.toBeInTheDocument()
    })
    expect(document.body.textContent).not.toContain('zhangsan')
    expect(document.body.textContent).not.toContain('7')
  })
})
