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
import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zh from '@/i18n/locales/zh.json'
import zhTW from '@/i18n/locales/zh-TW.json'

import {
  formatModelCapabilityJson,
  isPositiveSafeInteger,
  listDiscardedCapabilityValues,
  recoverModelCapability,
  parseModelCapabilityJson,
  serializeModelCapability,
} from '../model-capabilities'

const full = {
  context_length: 1050000,
  top_provider: { max_completion_tokens: 128000 },
  architecture: {
    input_modalities: ['text', 'image'],
    output_modalities: ['text'],
  },
  reasoning: { supported_efforts: ['none', 'low', 'medium', 'high', 'xhigh', 'max'] },
}

function errorPaths(text: string) {
  const result = parseModelCapabilityJson(text)
  expect(result.ok).toBe(false)
  return result.ok ? [] : result.errors.map((e) => e.path)
}

describe('isPositiveSafeInteger', () => {
  it.each([1, 128000, 9007199254740991])('accepts %s', (n) => {
    expect(isPositiveSafeInteger(n)).toBe(true)
  })
  it.each([0, -5, 1.5, Number.NaN, Number.POSITIVE_INFINITY, 9007199254740992, '12', 'abc', null])(
    'rejects %s',
    (n) => {
      expect(isPositiveSafeInteger(n)).toBe(false)
    }
  )
})

describe('parseModelCapabilityJson', () => {
  it('accepts the full shape and round-trips it canonically', () => {
    const result = parseModelCapabilityJson(JSON.stringify(full))
    expect(result.ok).toBe(true)
    if (!result.ok) return
    expect(JSON.parse(serializeModelCapability(result.value))).toEqual(full)
  })

  it('treats empty object as unconfigured', () => {
    const result = parseModelCapabilityJson('{}')
    expect(result.ok).toBe(true)
    if (!result.ok) return
    expect(serializeModelCapability(result.value)).toBe('{}')
  })

  it('treats null fields and empty modality arrays as unconfigured', () => {
    const result = parseModelCapabilityJson(
      JSON.stringify({
        context_length: null,
        top_provider: { max_completion_tokens: null },
        architecture: { input_modalities: [], output_modalities: null },
        reasoning: null,
      })
    )
    expect(result.ok).toBe(true)
    if (!result.ok) return
    expect(serializeModelCapability(result.value)).toBe('{}')
  })

  it('keeps an empty supported_efforts as "not supported"', () => {
    const result = parseModelCapabilityJson(
      '{"reasoning":{"supported_efforts":[]}}'
    )
    expect(result.ok).toBe(true)
    if (!result.ok) return
    expect(serializeModelCapability(result.value)).toBe(
      '{"reasoning":{"supported_efforts":[]}}'
    )
  })

  it('sorts and dedupes efforts and orders keys canonically', () => {
    const result = parseModelCapabilityJson(
      '{"reasoning":{"supported_efforts":["max","low","low","none"]},"architecture":{"output_modalities":["text"]},"top_provider":{"max_completion_tokens":5},"context_length":9}'
    )
    expect(result.ok).toBe(true)
    if (!result.ok) return
    expect(serializeModelCapability(result.value)).toBe(
      '{"context_length":9,"top_provider":{"max_completion_tokens":5},"architecture":{"output_modalities":["text"]},"reasoning":{"supported_efforts":["none","low","max"]}}'
    )
  })

  it('accepts the safe integer upper bound', () => {
    expect(
      parseModelCapabilityJson('{"context_length":9007199254740991}').ok
    ).toBe(true)
  })

  it('rejects malformed JSON', () => {
    expect(parseModelCapabilityJson('{"context_length":').ok).toBe(false)
    expect(parseModelCapabilityJson('').ok).toBe(false)
  })

  it('reports unknown modality with its path', () => {
    expect(
      errorPaths('{"architecture":{"input_modalities":["text","video"]}}')
    ).toContain('architecture.input_modalities')
  })

  it('reports unknown effort with its path', () => {
    expect(
      errorPaths('{"reasoning":{"supported_efforts":["turbo"]}}')
    ).toContain('reasoning.supported_efforts')
  })

  it('reports extra fields at top level and nested', () => {
    expect(errorPaths('{"foo":1}')).toContain('foo')
    expect(errorPaths('{"reasoning":{"bar":1}}')).toContain('reasoning.bar')
  })

  it.each(['0', '-5', '1.5', '"abc"', '9007199254740992'])(
    'reports out-of-range context_length %s',
    (v) => {
      expect(errorPaths(`{"context_length":${v}}`)).toContain('context_length')
    }
  )

  it('reports out-of-range max_completion_tokens', () => {
    expect(
      errorPaths('{"top_provider":{"max_completion_tokens":0}}')
    ).toContain('top_provider.max_completion_tokens')
  })

  it('reports every bad field, not just the first', () => {
    const paths = errorPaths(
      '{"context_length":0,"architecture":{"input_modalities":["x"]}}'
    )
    expect(paths).toEqual(
      expect.arrayContaining(['context_length', 'architecture.input_modalities'])
    )
  })
})

describe('serializeModelCapability', () => {
  it('serializes unconfigured values as {}', () => {
    const result = parseModelCapabilityJson('{}')
    expect(result.ok && serializeModelCapability(result.value)).toBe('{}')
  })
})

describe('formatModelCapabilityJson', () => {
  it('writes two-space indented lines with each array on one line', () => {
    expect(
      formatModelCapabilityJson({
        context_length: 1050000,
        architecture: { input_modalities: ['text', 'image'] },
        reasoning: { supported_efforts: [] },
      })
    ).toBe(
      [
        '{',
        '  "context_length": 1050000,',
        '  "architecture": {',
        '    "input_modalities": ["text", "image"]',
        '  },',
        '  "reasoning": {',
        '    "supported_efforts": []',
        '  }',
        '}',
      ].join('\n')
    )
  })

  it('writes an empty object as {} and stays parseable', () => {
    expect(formatModelCapabilityJson({})).toBe('{}')
    expect(JSON.parse(formatModelCapabilityJson(full))).toEqual(full)
  })
})

describe('recoverModelCapability', () => {
  it('keeps recognised fields and values and drops the rest', () => {
    expect(
      recoverModelCapability(
        JSON.stringify({
          context_length: 1050000,
          top_provider: { max_completion_tokens: 128000, other: 1 },
          architecture: {
            input_modalities: ['text', 'video', 'image'],
            output_modalities: ['video'],
          },
          reasoning: { supported_efforts: ['low', 'ultra'] },
          unknown_field: true,
        })
      )
    ).toEqual({
      context_length: 1050000,
      top_provider: { max_completion_tokens: 128000 },
      architecture: { input_modalities: ['text', 'image'] },
      reasoning: { supported_efforts: ['low'] },
    })
  })

  it('drops numbers that are not positive safe integers', () => {
    expect(
      recoverModelCapability(
        '{"context_length":-1,"top_provider":{"max_completion_tokens":1.5}}'
      )
    ).toEqual({})
  })

  it('keeps an empty supported_efforts as "not supported"', () => {
    expect(
      recoverModelCapability('{"reasoning":{"supported_efforts":[]}}')
    ).toEqual({ reasoning: { supported_efforts: [] } })
  })

  it.each(['{"context_length":', '[]', '"x"', 'null'])(
    'returns null for %s',
    (text) => {
      expect(recoverModelCapability(text)).toBeNull()
    }
  )
})

describe('listDiscardedCapabilityValues', () => {
  it('lists only the non-string item of a mixed array', () => {
    expect(
      listDiscardedCapabilityValues(
        '{"architecture":{"input_modalities":[1,"text"]}}'
      )
    ).toEqual([
      { path: 'architecture.input_modalities', value: '1', scope: 'item' },
    ])
  })

  it('lists a repeated unknown item once', () => {
    expect(
      listDiscardedCapabilityValues(
        '{"architecture":{"input_modalities":["video","video"]}}'
      )
    ).toEqual([
      { path: 'architecture.input_modalities', value: 'video', scope: 'item' },
    ])
  })

  it('lists whole fields with JSON-written values and single items by name', () => {
    expect(
      listDiscardedCapabilityValues(
        '{"context_length":-1,"reasoning":{"supported_efforts":["low","ultra"]},"foo":"bar"}'
      )
    ).toEqual([
      { path: 'context_length', value: '-1', scope: 'field' },
      { path: 'reasoning.supported_efforts', value: 'ultra', scope: 'item' },
      { path: 'foo', value: '"bar"', scope: 'field' },
    ])
  })

  it('lists nothing for recognised values or invalid JSON', () => {
    expect(listDiscardedCapabilityValues(JSON.stringify(full))).toEqual([])
    expect(listDiscardedCapabilityValues('{"context_length":')).toEqual([])
  })
})

describe('model capability locale keys', () => {
  const keys = [
    'Model capabilities',
    '{{path}} contains an unknown value {{value}}. Allowed values: {{allowed}}',
    '{{path}} must be an integer from 1 to 9007199254740991',
    '{{path}} is not a capability field',
    '{{path}} must be an object',
    '{{path}} must be an array of strings',
    'Model name must be 1 to 128 characters',
    '{{name}} is used by a prefix, suffix, or contains rule record. Edit that record in the drawer.',
    'Model metadata changed. Preview again before saving.',
  ]
  const locales = { en, zh, 'zh-TW': zhTW, fr, ru, ja, vi }
  for (const [name, dict] of Object.entries(locales)) {
    it(`${name} has every key`, () => {
      const missing = keys.filter((k) => !(k in dict.translation))
      expect(missing).toEqual([])
    })
  }
})
