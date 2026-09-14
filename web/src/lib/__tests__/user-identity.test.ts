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
import i18next from 'i18next'
import { describe, expect, test } from 'vitest'

import {
  describeUserTooltip,
  resolveUserName,
  type UserIdentity,
} from '../user-identity'

const t = i18next.t

describe('resolveUserName', () => {
  test.each<[string, UserIdentity, string]>([
    [
      'prefers display_name over username',
      { user_id: 7, username: 'zhangsan', display_name: '张三' },
      '张三',
    ],
    [
      'falls back to username when display_name is whitespace only',
      { user_id: 7, username: 'zhangsan', display_name: '  ' },
      'zhangsan',
    ],
    [
      'shows the name once when display_name equals username',
      { user_id: 7, username: 'zhangsan', display_name: 'zhangsan' },
      'zhangsan',
    ],
    [
      'falls back to the localized placeholder when neither name is set',
      { user_id: 7 },
      'User 7',
    ],
    [
      'falls back to the localized placeholder when username is empty string',
      { user_id: 7, username: '' },
      'User 7',
    ],
  ])('%s', (_label, user, expected) => {
    expect(resolveUserName(user, t)).toBe(expected)
  })
})

describe('describeUserTooltip', () => {
  test('includes both username and user id lines when username is present', () => {
    expect(
      describeUserTooltip(
        { user_id: 7, username: 'zhangsan', display_name: '张三' },
        t
      )
    ).toEqual(['Username: zhangsan', 'User ID: 7'])
  })

  test('omits the username line when username is missing', () => {
    expect(describeUserTooltip({ user_id: 7 }, t)).toEqual(['User ID: 7'])
  })

  test('omits the username line when username is an empty string', () => {
    expect(describeUserTooltip({ user_id: 7, username: '' }, t)).toEqual([
      'User ID: 7',
    ])
  })
})
