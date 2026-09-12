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
import type { TFunction } from 'i18next'

interface UserIdentity {
  user_id: number
  username?: string
  display_name?: string
}

function resolveUserName(user: UserIdentity, t: TFunction): string {
  const displayName = user.display_name?.trim()
  if (displayName) return displayName
  if (user.username) return user.username
  return t('User {{id}}', { id: user.user_id })
}

function describeUserTooltip(user: UserIdentity, t: TFunction): string[] {
  const lines: string[] = []
  if (user.username) {
    lines.push(t('Username: {{name}}', { name: user.username }))
  }
  lines.push(t('User ID: {{id}}', { id: user.user_id }))
  return lines
}

export { resolveUserName, describeUserTooltip, type UserIdentity }
