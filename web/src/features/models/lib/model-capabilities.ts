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
export const MODEL_INPUT_MODALITIES = [
  'text',
  'image',
  'file',
  'audio',
] as const
export const MODEL_OUTPUT_MODALITIES = ['text', 'image', 'audio'] as const
export const MODEL_REASONING_EFFORTS = [
  'none',
  'minimal',
  'low',
  'medium',
  'high',
  'xhigh',
  'max',
] as const

// 数字字段保留 string：可视表单里没填对的输入要原样带进校验，才能标红并拦住保存
export type ModelCapability = {
  context_length?: number | string
  top_provider?: { max_completion_tokens?: number | string }
  architecture?: {
    input_modalities?: string[]
    output_modalities?: string[]
  }
  reasoning?: { supported_efforts?: string[] }
}

export type ModelCapabilityError = {
  path: string
  key: string
  params: Record<string, string>
}

export type ModelCapabilityParseResult =
  | { ok: true; value: ModelCapability }
  | { ok: false; errors: ModelCapabilityError[] }

export type ModelCapabilityFormValues = {
  context_length: string
  max_completion_tokens: string
  input_modalities: string[]
  output_modalities: string[]
  supported_efforts: string[]
  no_reasoning: boolean
}

const TOP_LEVEL_FIELDS = new Set([
  'context_length',
  'top_provider',
  'architecture',
  'reasoning',
])
const NESTED_FIELDS: Record<string, readonly string[]> = {
  top_provider: ['max_completion_tokens'],
  architecture: ['input_modalities', 'output_modalities'],
  reasoning: ['supported_efforts'],
}
const ARRAY_OPTIONS: Record<string, readonly string[]> = {
  'architecture.input_modalities': MODEL_INPUT_MODALITIES,
  'architecture.output_modalities': MODEL_OUTPUT_MODALITIES,
  'reasoning.supported_efforts': MODEL_REASONING_EFFORTS,
}

export function isPositiveSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 1
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function sortByOptions(values: string[], options: readonly string[]) {
  return [...new Set(values)].sort(
    (a, b) => options.indexOf(a) - options.indexOf(b)
  )
}

function checkInteger(
  value: unknown,
  path: string,
  errors: ModelCapabilityError[]
) {
  if (value === null || value === undefined) return
  if (!isPositiveSafeInteger(value)) {
    errors.push({
      path,
      key: '{{path}} must be an integer from 1 to 9007199254740991',
      params: { path },
    })
  }
}

function checkArray(
  value: unknown,
  path: string,
  errors: ModelCapabilityError[]
) {
  if (value === null || value === undefined) return
  if (!Array.isArray(value) || value.some((v) => typeof v !== 'string')) {
    errors.push({
      path,
      key: '{{path}} must be an array of strings',
      params: { path },
    })
    return
  }
  const allowed = ARRAY_OPTIONS[path]
  const unknown = value.filter((v) => !allowed.includes(v))
  if (unknown.length > 0) {
    errors.push({
      path,
      key: '{{path}} contains an unknown value {{value}}. Allowed values: {{allowed}}',
      params: { path, value: unknown.join(', '), allowed: allowed.join(', ') },
    })
  }
}

function collectErrors(root: Record<string, unknown>) {
  const errors: ModelCapabilityError[] = []
  const notField = (path: string) =>
    errors.push({
      path,
      key: '{{path}} is not a capability field',
      params: { path },
    })

  for (const [name, value] of Object.entries(root)) {
    if (!TOP_LEVEL_FIELDS.has(name)) {
      notField(name)
    } else if (name === 'context_length') {
      checkInteger(value, name, errors)
    } else if (value !== null && !isPlainObject(value)) {
      errors.push({
        path: name,
        key: '{{path}} must be an object',
        params: { path: name },
      })
    } else if (isPlainObject(value)) {
      for (const [child, childValue] of Object.entries(value)) {
        const path = `${name}.${child}`
        if (!NESTED_FIELDS[name].includes(child)) notField(path)
        else if (name === 'top_provider') checkInteger(childValue, path, errors)
        else checkArray(childValue, path, errors)
      }
    }
  }
  return errors
}

export function normalizeModelCapability(
  raw: ModelCapability
): ModelCapability {
  const result: ModelCapability = {}
  if (raw.context_length != null) result.context_length = raw.context_length
  const maxTokens = raw.top_provider?.max_completion_tokens
  if (maxTokens != null) {
    result.top_provider = { max_completion_tokens: maxTokens }
  }
  const input = raw.architecture?.input_modalities ?? []
  const output = raw.architecture?.output_modalities ?? []
  if (input.length > 0 || output.length > 0) {
    result.architecture = {}
    if (input.length > 0) {
      result.architecture.input_modalities = sortByOptions(
        input,
        MODEL_INPUT_MODALITIES
      )
    }
    if (output.length > 0) {
      result.architecture.output_modalities = sortByOptions(
        output,
        MODEL_OUTPUT_MODALITIES
      )
    }
  }
  if (raw.reasoning?.supported_efforts != null) {
    result.reasoning = {
      supported_efforts: sortByOptions(
        raw.reasoning.supported_efforts,
        MODEL_REASONING_EFFORTS
      ),
    }
  }
  return result
}

export function parseModelCapabilityJson(
  text: string
): ModelCapabilityParseResult {
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    return {
      ok: false,
      errors: [{ path: '$', key: 'Invalid JSON', params: {} }],
    }
  }
  if (!isPlainObject(parsed)) {
    return {
      ok: false,
      errors: [
        { path: '$', key: '{{path}} must be an object', params: { path: '$' } },
      ],
    }
  }
  const errors = collectErrors(parsed)
  if (errors.length > 0) return { ok: false, errors }
  return {
    ok: true,
    value: normalizeModelCapability(parsed as ModelCapability),
  }
}

export function serializeModelCapability(value: ModelCapability): string {
  return JSON.stringify(normalizeModelCapability(value))
}

export function capabilityToFormValues(
  value: ModelCapability
): ModelCapabilityFormValues {
  const efforts = value.reasoning?.supported_efforts
  return {
    context_length: String(value.context_length ?? ''),
    max_completion_tokens: String(
      value.top_provider?.max_completion_tokens ?? ''
    ),
    input_modalities: value.architecture?.input_modalities ?? [],
    output_modalities: value.architecture?.output_modalities ?? [],
    supported_efforts: efforts ?? [],
    no_reasoning: efforts !== undefined && efforts.length === 0,
  }
}

function numberOrRaw(text: string): number | string | undefined {
  const trimmed = text.trim()
  if (trimmed === '') return undefined
  const num = Number(trimmed)
  return Number.isFinite(num) ? num : trimmed
}

export function formValuesToCapability(
  values: ModelCapabilityFormValues
): ModelCapability {
  const result: ModelCapability = {}
  const contextLength = numberOrRaw(values.context_length)
  if (contextLength !== undefined) result.context_length = contextLength
  const maxTokens = numberOrRaw(values.max_completion_tokens)
  if (maxTokens !== undefined) {
    result.top_provider = { max_completion_tokens: maxTokens }
  }
  result.architecture = {
    input_modalities: values.input_modalities,
    output_modalities: values.output_modalities,
  }
  if (values.no_reasoning) {
    result.reasoning = { supported_efforts: [] }
  } else if (values.supported_efforts.length > 0) {
    result.reasoning = { supported_efforts: values.supported_efforts }
  }
  return normalizeModelCapability(result)
}

// 空串和 {} 都存成空串，后端据此认作未配置
export function capabilityTextToPayload(text: string): string {
  if (text.trim() === '') return ''
  const result = parseModelCapabilityJson(text)
  if (!result.ok) return text
  const serialized = serializeModelCapability(result.value)
  return serialized === '{}' ? '' : serialized
}
