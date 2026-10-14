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
import { describe, expect, test } from 'vitest'

import { convertDetectedLanguage } from './languages'
import en from './locales/en.json'
import fr from './locales/fr.json'
import ja from './locales/ja.json'
import ru from './locales/ru.json'
import vi from './locales/vi.json'
import zh from './locales/zh.json'
import zhTW from './locales/zh-TW.json'

describe('convertDetectedLanguage', () => {
  test('maps browser BCP-47 Chinese tags onto interface codes', () => {
    expect(convertDetectedLanguage('zh-TW')).toBe('zhTW')
    expect(convertDetectedLanguage('zh-HK')).toBe('zhTW')
    expect(convertDetectedLanguage('zh-MO')).toBe('zhTW')
    expect(convertDetectedLanguage('zh-Hant-TW')).toBe('zhTW')
    expect(convertDetectedLanguage('zh')).toBe('zhCN')
    expect(convertDetectedLanguage('zh-CN')).toBe('zhCN')
    expect(convertDetectedLanguage('zh-Hans')).toBe('zhCN')
  })

  test('keeps already-normalized interface codes stable (localStorage round-trip)', () => {
    // i18next caches `zhTW`/`zhCN` (the supportedLngs codes) to localStorage,
    // and the detector runs this converter on the cached value at every page
    // load — if `zhTW` does not survive the round-trip, a user who picked
    // Traditional Chinese is flipped to Simplified on the next load and the
    // cache is overwritten, making the flip permanent.
    expect(convertDetectedLanguage('zhTW')).toBe('zhTW')
    expect(convertDetectedLanguage('zhCN')).toBe('zhCN')
  })

  test('passes non-Chinese values through unchanged', () => {
    expect(convertDetectedLanguage('en')).toBe('en')
    expect(convertDetectedLanguage('fr-FR')).toBe('fr-FR')
    expect(convertDetectedLanguage('ja')).toBe('ja')
  })
})

describe('Claude subscription message keys', () => {
  const keys = [
    'Invalid parameters',
    'Channel does not exist',
    'Channel type is not Claude Subscription',
    'Multi-key channels are not supported',
    'Failed to parse credentials. Please check the channel configuration',
    'Invalid reset parameters',
    'Failed to fetch usage. Please try again later',
    'Reset succeeded',
    'This reset has already been used',
    'Not currently rate limited, no reset needed',
    'On cooldown, please try again later',
    'Account is not eligible',
    'Upstream temporarily unavailable',
    'Upstream returned an unknown result',
    'Too many requests, please try again later',
    'Invalid credentials or insufficient permissions',
    'Upstream returned HTTP {{status}}',
    'Reset outcome unknown, refresh usage to confirm before retrying',
    'Unsupported reset type',
    'Unable to get the latest Claude Code version, cannot perform reset',
    'Failed to get account organization info',
    'Reset failed, please try again later',
  ]
  const locales = { en, zh, 'zh-TW': zhTW, fr, ja, ru, vi }

  test.each(Object.entries(locales))('%s translates every key', (_, locale) => {
    const translation: Record<string, string> = locale.translation
    for (const key of keys) {
      expect(translation[key], key).toBeTruthy()
    }
    expect(translation['Upstream returned HTTP {{status}}']).toContain('{{status}}')
  })
})
