import { describe, expect, test } from 'vitest'

import { CHANNEL_FORM_DEFAULT_VALUES, channelFormSchema } from './channel-form'

const CLAUDE_SUBSCRIPTION_TYPE = 61

function hasIssueForField(
  issues: readonly { path: PropertyKey[] }[],
  field: string
): boolean {
  return issues.some((issue) => issue.path[0] === field)
}

describe('channelFormSchema for Claude subscription channel (type 61)', () => {
  test('accepts a valid Claude Code OAuth credential', () => {
    const result = channelFormSchema.safeParse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: CLAUDE_SUBSCRIPTION_TYPE,
      multi_key_mode: 'single',
      key: JSON.stringify({
        claudeAiOauth: {
          accessToken: 'sk-ant-oat01-x',
          refreshToken: 'r',
          expiresAt: 123,
        },
      }),
    })

    const keyIssues = result.success
      ? []
      : result.error.issues.filter((issue) => issue.path[0] === 'key')
    expect(keyIssues.length).toBe(0)
  })

  test('rejects a non-JSON key', () => {
    const result = channelFormSchema.safeParse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: CLAUDE_SUBSCRIPTION_TYPE,
      multi_key_mode: 'single',
      key: 'plain-not-json',
    })

    expect(result.success).toBe(false)
    if (!result.success) {
      expect(hasIssueForField(result.error.issues, 'key')).toBe(true)
    }
  })

  test('rejects JSON missing claudeAiOauth.accessToken', () => {
    const result = channelFormSchema.safeParse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: CLAUDE_SUBSCRIPTION_TYPE,
      multi_key_mode: 'single',
      key: JSON.stringify({ claudeAiOauth: { refreshToken: 'r' } }),
    })

    expect(result.success).toBe(false)
    if (!result.success) {
      expect(hasIssueForField(result.error.issues, 'key')).toBe(true)
    }
  })

  test('rejects batch/multi-key creation', () => {
    const result = channelFormSchema.safeParse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: CLAUDE_SUBSCRIPTION_TYPE,
      multi_key_mode: 'batch',
      key: JSON.stringify({
        claudeAiOauth: { accessToken: 'sk-ant-oat01-x' },
      }),
    })

    expect(result.success).toBe(false)
    if (!result.success) {
      expect(hasIssueForField(result.error.issues, 'multi_key_mode')).toBe(true)
    }
  })
})
