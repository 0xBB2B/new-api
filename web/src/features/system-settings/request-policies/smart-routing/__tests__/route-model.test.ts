import type { TFunction } from 'i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { getBuiltinThinkingCapabilities, validateSmartRoute } from '../api'
import {
  clampEffort,
  effectiveCapability,
  effortOverflow,
  tierModelsOfRoutes,
} from '../capability'
import {
  endpointShortNames,
  parseThinkingCapabilities,
  serializeThinkingCapabilities,
  slotSummaries,
  SMART_THINKING_CAPABILITIES_OPTION_KEY,
  tierEffortSummary,
} from '../lib'
import { createRouteFormSchema, emptyRouteFormValues } from '../route-form'
import type {
  SmartEffort,
  SmartRoute,
  SmartRouteTier,
  ThinkingCapabilityOverride,
} from '../types'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }))

const t = ((key: string, params?: Record<string, unknown>) =>
  key.replaceAll(/\{\{(\w+)\}\}/g, (_, name: string) =>
    String(params?.[name] ?? '')
  )) as unknown as TFunction

function tier(overrides: Partial<SmartRouteTier> = {}): SmartRouteTier {
  return {
    model: 'gpt-5.5',
    groups: ['api'],
    effort_mode: '',
    effort: '',
    effort_min: '',
    effort_max: '',
    ...overrides,
  }
}

function validValues(): SmartRoute {
  return {
    ...emptyRouteFormValues(),
    name: 'auto-gpt',
    endpoints: ['openai-response'],
    tiers: {
      simple: tier(),
      normal: tier(),
      hard: tier(),
    },
    scorer: { model: 's', groups: ['api'], timeout_ms: 2000 },
    advisor: { model: 'a', groups: ['api'], after_dissatisfied: 2 },
  }
}

function route(
  models: Record<string, string>,
  scorer = '',
  advisor = ''
): SmartRoute {
  return {
    ...validValues(),
    tiers: {
      simple: tier({ model: models.simple ?? '' }),
      normal: tier({ model: models.normal ?? '' }),
      hard: tier({ model: models.hard ?? '' }),
    },
    scorer: { model: scorer, groups: [], timeout_ms: 2000 },
    advisor: { model: advisor, groups: [], after_dissatisfied: 2 },
  }
}

const cap = (efforts: SmartEffort[], noThinking = false) => ({
  source: 'builtin' as const,
  noThinking,
  efforts,
})

describe('emptyRouteFormValues', () => {
  it('new route defaults to responses endpoint and no effort override in all tiers', () => {
    const values = emptyRouteFormValues()
    expect(values.endpoints).toEqual(['openai-response'])
    for (const id of ['simple', 'normal', 'hard'] as const) {
      expect(values.tiers[id].effort_mode).toBe('')
    }
  })
})

describe('createRouteFormSchema', () => {
  const schema = createRouteFormSchema(t)

  it('accepts a valid route', () => {
    expect(schema.safeParse(validValues()).success).toBe(true)
  })

  it('rejects empty endpoints with the endpoint message', () => {
    const result = schema.safeParse({ ...validValues(), endpoints: [] })
    expect(result.success).toBe(false)
    expect(result.error?.issues.map((i) => i.message)).toContain(
      'Select at least one endpoint'
    )
  })

  it('rejects fixed mode without an effort value', () => {
    const values = validValues()
    values.tiers.normal = tier({ effort_mode: 'fixed', effort: '' })
    const result = schema.safeParse(values)
    expect(result.success).toBe(false)
    expect(result.error?.issues.some((i) => i.path[1] === 'normal')).toBe(true)
  })

  it('accepts fixed mode with an effort value', () => {
    const values = validValues()
    values.tiers.normal = tier({ effort_mode: 'fixed', effort: 'medium' })
    expect(schema.safeParse(values).success).toBe(true)
  })

  it('rejects follow mode whose min is above max', () => {
    const values = validValues()
    values.tiers.hard = tier({
      effort_mode: 'follow',
      effort_min: 'high',
      effort_max: 'low',
    })
    const result = schema.safeParse(values)
    expect(result.success).toBe(false)
    expect(result.error?.issues.some((i) => i.path[1] === 'hard')).toBe(true)
  })

  it.each([
    ['none mode with effort', { effort: 'low' }],
    ['none mode with effort_min', { effort_min: 'low' }],
    ['none mode with effort_max', { effort_max: 'low' }],
    [
      'fixed mode with effort_min',
      { effort_mode: 'fixed', effort: 'low', effort_min: 'low' },
    ],
    [
      'fixed mode with effort_max',
      { effort_mode: 'fixed', effort: 'low', effort_max: 'low' },
    ],
    [
      'follow mode with effort',
      {
        effort_mode: 'follow',
        effort_min: 'low',
        effort_max: 'high',
        effort: 'low',
      },
    ],
    ['follow mode without min', { effort_mode: 'follow', effort_max: 'high' }],
    ['follow mode without max', { effort_mode: 'follow', effort_min: 'low' }],
  ] as [string, Partial<SmartRouteTier>][])(
    'rejects %s with the valid-effort message',
    (_, overrides) => {
      const values = validValues()
      values.tiers.normal = tier(overrides)
      const result = schema.safeParse(values)
      expect(result.success).toBe(false)
      expect(result.error?.issues.map((i) => i.message)).toContain(
        'Select a valid reasoning effort'
      )
    }
  )

  it('accepts follow mode with min equal to max', () => {
    const values = validValues()
    values.tiers.hard = tier({
      effort_mode: 'follow',
      effort_min: 'high',
      effort_max: 'high',
    })
    expect(schema.safeParse(values).success).toBe(true)
  })

  it('reports min above max on effort_max with the ordering message', () => {
    const values = validValues()
    values.tiers.hard = tier({
      effort_mode: 'follow',
      effort_min: 'high',
      effort_max: 'low',
    })
    const issue = schema
      .safeParse(values)
      .error?.issues.find((i) => i.path[1] === 'hard')
    expect(issue?.path[2]).toBe('effort_max')
    expect(issue?.message).toBe(
      'The minimum effort cannot be higher than the maximum'
    )
  })

  it('accepts follow mode with min below max', () => {
    const values = validValues()
    values.tiers.hard = tier({
      effort_mode: 'follow',
      effort_min: 'medium',
      effort_max: 'xhigh',
    })
    expect(schema.safeParse(values).success).toBe(true)
  })
})

describe('thinking capabilities option', () => {
  it('uses the smart routing option key', () => {
    expect(SMART_THINKING_CAPABILITIES_OPTION_KEY).toBe(
      'smart_routing_setting.thinking_capabilities'
    )
  })

  it('round-trips through serialize and parse', () => {
    const map: Record<string, ThinkingCapabilityOverride> = {
      'glm-5.1': { no_thinking: false, efforts: ['low', 'high'] },
    }
    expect(
      parseThinkingCapabilities(serializeThinkingCapabilities(map))
    ).toEqual(map)
  })

  it('parses empty text to an empty object', () => {
    expect(parseThinkingCapabilities('')).toEqual({})
  })

  it('parses invalid JSON to an empty object', () => {
    expect(parseThinkingCapabilities('{oops')).toEqual({})
  })
})

describe('endpointShortNames', () => {
  it('joins responses and anthropic with a slash', () => {
    expect(endpointShortNames(['openai-response', 'anthropic'])).toBe(
      'Responses / Anthropic'
    )
  })

  it('names openai as Chat', () => {
    expect(endpointShortNames(['openai'])).toBe('Chat')
  })
})

describe('tierEffortSummary', () => {
  it('says Not overridden when the tier has no effort mode', () => {
    expect(tierEffortSummary(tier(), cap(['low', 'medium']), t)).toBe(
      'Not overridden'
    )
  })

  it('shows the fixed effort', () => {
    expect(
      tierEffortSummary(
        tier({ effort_mode: 'fixed', effort: 'medium' }),
        cap(['low', 'medium']),
        t
      )
    ).toBe('Fixed medium')
  })

  it('shows the follow range', () => {
    expect(
      tierEffortSummary(
        tier({
          effort_mode: 'follow',
          effort_min: 'medium',
          effort_max: 'xhigh',
        }),
        cap(['medium', 'xhigh']),
        t
      )
    ).toBe('Follow scoring medium～xhigh')
  })

  it.each([
    ['not overridden', tier()],
    [
      'follow',
      tier({ effort_mode: 'follow', effort_min: 'low', effort_max: 'high' }),
    ],
  ])('says No thinking for a no-thinking model in %s mode', (_, row) => {
    expect(tierEffortSummary(row, cap([], true), t)).toBe('No thinking')
  })

  it('says No thinking when the model cannot think', () => {
    expect(
      tierEffortSummary(
        tier({ effort_mode: 'fixed', effort: 'medium' }),
        cap([], true),
        t
      )
    ).toBe('No thinking')
  })
})

describe('slotSummaries', () => {
  it('reports unset slots for a new route', () => {
    expect(slotSummaries(route({}), t)).toEqual({
      scorer: 'Not set',
      tiers: '0/3 set',
      advisor: 'Not set',
    })
  })

  it('counts tiers with a model', () => {
    expect(
      slotSummaries(route({ simple: 'a', normal: 'b', hard: 'c' }), t).tiers
    ).toBe('3/3 set')
  })

  it('shows scorer and advisor model names', () => {
    const summary = slotSummaries(route({}, 'gpt-5.6-luna', 'gpt-5.6-sol'), t)
    expect(summary.scorer).toBe('gpt-5.6-luna')
    expect(summary.advisor).toBe('gpt-5.6-sol')
  })
})

describe('effectiveCapability', () => {
  const builtin: Record<
    string,
    { known: boolean; no_thinking: boolean; efforts: SmartEffort[] }
  > = {
    'gpt-5.5': { known: true, no_thinking: false, efforts: ['low', 'high'] },
    mystery: { known: false, no_thinking: false, efforts: [] },
  }

  it('uses the builtin capability when there is no override', () => {
    expect(effectiveCapability('gpt-5.5', builtin, {})).toEqual({
      source: 'builtin',
      noThinking: false,
      efforts: ['low', 'high'],
    })
  })

  it('prefers the override over builtin', () => {
    expect(
      effectiveCapability('gpt-5.5', builtin, {
        'gpt-5.5': { no_thinking: false, efforts: ['medium'] },
      })
    ).toEqual({ source: 'override', noThinking: false, efforts: ['medium'] })
  })

  it('is unconfirmed when builtin marks the model unknown', () => {
    expect(effectiveCapability('mystery', builtin, {})).toEqual({
      source: 'unconfirmed',
      noThinking: false,
      efforts: ['low', 'medium', 'high'],
    })
  })

  it('is unconfirmed when builtin has no entry for the model', () => {
    expect(effectiveCapability('absent', builtin, {}).source).toBe(
      'unconfirmed'
    )
  })
})

describe('clampEffort', () => {
  it('keeps an effort the model supports', () => {
    expect(clampEffort('medium', cap(['low', 'medium', 'high']))).toEqual({
      value: 'medium',
      changed: false,
    })
  })

  it('moves to the nearest supported level', () => {
    expect(clampEffort('xhigh', cap(['low', 'medium', 'high']))).toEqual({
      value: 'high',
      changed: true,
    })
  })

  it('picks the higher level when two are equally near', () => {
    expect(clampEffort('medium', cap(['low', 'high']))).toEqual({
      value: 'high',
      changed: true,
    })
  })

  it('moves none to the lowest supported level', () => {
    expect(clampEffort('none', cap(['low', 'high']))).toEqual({
      value: 'low',
      changed: true,
    })
  })

  it('returns an empty value when the model cannot think', () => {
    expect(clampEffort('medium', cap([], true)).value).toBe('')
  })
})

describe('effortOverflow', () => {
  it('reports overflow when follow max exceeds the model ceiling', () => {
    expect(
      effortOverflow(
        tier({
          effort_mode: 'follow',
          effort_min: 'medium',
          effort_max: 'xhigh',
        }),
        cap(['low', 'medium', 'high'])
      )
    ).toEqual({ overflow: true, actual: 'high' })
  })

  it('reports no overflow for a fixed value within capability', () => {
    expect(
      effortOverflow(
        tier({ effort_mode: 'fixed', effort: 'low' }),
        cap(['low', 'medium', 'high'])
      ).overflow
    ).toBe(false)
  })

  it('reports overflow when a fixed value exceeds the model ceiling', () => {
    expect(
      effortOverflow(
        tier({ effort_mode: 'fixed', effort: 'xhigh' }),
        cap(['low', 'medium', 'high'])
      )
    ).toEqual({ overflow: true, actual: 'high' })
  })

  it('reports overflow when follow min is out of range and settles on the max', () => {
    expect(
      effortOverflow(
        tier({ effort_mode: 'follow', effort_min: 'none', effort_max: 'high' }),
        cap(['low', 'medium', 'high'])
      )
    ).toEqual({ overflow: true, actual: 'high' })
  })

  it('reports overflow against an unconfirmed model ceiling', () => {
    expect(
      effortOverflow(
        tier({
          effort_mode: 'follow',
          effort_min: 'medium',
          effort_max: 'xhigh',
        }),
        {
          source: 'unconfirmed',
          noThinking: false,
          efforts: ['low', 'medium', 'high'],
        }
      )
    ).toEqual({ overflow: true, actual: 'high' })
  })

  it('reports no overflow when the model cannot think', () => {
    expect(
      effortOverflow(
        tier({ effort_mode: 'fixed', effort: 'high' }),
        cap([], true)
      ).overflow
    ).toBe(false)
  })
})

describe('tierModelsOfRoutes', () => {
  it('dedupes tier models in first-seen order and skips scorer and advisor', () => {
    const routes = [
      route({ simple: 'b', normal: 'a', hard: 'b' }, 'scorer-m', 'advisor-m'),
      route({ simple: 'c', normal: 'a', hard: 'd' }, 'scorer-m', 'advisor-m'),
    ]
    expect(tierModelsOfRoutes(routes)).toEqual(['b', 'a', 'c', 'd'])
  })
})

describe('smart routing api', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
    vi.mocked(api.post).mockReset()
  })

  it('requests builtin capabilities with comma-joined models', async () => {
    const data = { 'gpt-5.5': { known: true, no_thinking: false, efforts: [] } }
    vi.mocked(api.get).mockResolvedValue({ data: { success: true, data } })
    const result = await getBuiltinThinkingCapabilities(['gpt-5.5', 'glm-5.1'])
    expect(api.get).toHaveBeenCalledWith(
      '/api/option/smart_routing/thinking_capabilities',
      { params: { models: 'gpt-5.5,glm-5.1' } }
    )
    expect(result).toEqual(data)
  })

  it('validateSmartRoute returns data containing only errors', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, data: { errors: [] } },
    })
    const result = await validateSmartRoute({ route: validValues() })
    expect(result).toEqual({ errors: [] })
  })
})
