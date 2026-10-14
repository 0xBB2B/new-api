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
import type { UserUtilization } from '../types'

export type UtilizationSegmentKey =
  | 'deep'
  | 'frequent'
  | 'moderate'
  | 'inactive'

export function buildUtilizationView(data: UserUtilization) {
  const total = data.total_users
  const percentOf = (count: number) =>
    total > 0 ? `${Math.round((count * 100) / total)}%` : '—'
  const frequentOnly = Math.max(data.frequent_users - data.deep_users, 0)

  const segments: {
    key: UtilizationSegmentKey
    count: number
    percent: string
  }[] = [
    {
      key: 'deep',
      count: data.deep_users,
      percent: percentOf(data.deep_users),
    },
    { key: 'frequent', count: frequentOnly, percent: percentOf(frequentOnly) },
    {
      key: 'moderate',
      count: data.moderate_users,
      percent: percentOf(data.moderate_users),
    },
    {
      key: 'inactive',
      count: data.inactive_users,
      percent: percentOf(data.inactive_users),
    },
  ]

  return {
    total,
    cards: {
      deep: { count: data.deep_users, percent: percentOf(data.deep_users) },
      frequent: {
        count: data.frequent_users,
        percent: percentOf(data.frequent_users),
      },
      inactive: {
        count: data.inactive_users,
        percent: percentOf(data.inactive_users),
      },
    },
    segments,
  }
}
