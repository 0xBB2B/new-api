import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
} from './channel-form'

const CLAUDE_SUBSCRIPTION_TYPE = 59

function hasIssueForField(
  issues: readonly { path: PropertyKey[] }[],
  field: string
): boolean {
  return issues.some((issue) => issue.path[0] === field)
}

describe('channelFormSchema for Claude subscription channel (type 59)', () => {
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
    assert.equal(keyIssues.length, 0)
  })

  test('rejects a non-JSON key', () => {
    const result = channelFormSchema.safeParse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: CLAUDE_SUBSCRIPTION_TYPE,
      multi_key_mode: 'single',
      key: 'plain-not-json',
    })

    assert.equal(result.success, false)
    if (!result.success) {
      assert.equal(hasIssueForField(result.error.issues, 'key'), true)
    }
  })

  test('rejects JSON missing claudeAiOauth.accessToken', () => {
    const result = channelFormSchema.safeParse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: CLAUDE_SUBSCRIPTION_TYPE,
      multi_key_mode: 'single',
      key: JSON.stringify({ claudeAiOauth: { refreshToken: 'r' } }),
    })

    assert.equal(result.success, false)
    if (!result.success) {
      assert.equal(hasIssueForField(result.error.issues, 'key'), true)
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

    assert.equal(result.success, false)
    if (!result.success) {
      assert.equal(hasIssueForField(result.error.issues, 'multi_key_mode'), true)
    }
  })
})
