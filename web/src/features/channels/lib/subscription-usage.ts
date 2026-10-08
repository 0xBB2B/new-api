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
import { formatTimestampToDate } from '@/lib/format'

export type CodexRateLimitWindow = {
  used_percent?: number
  reset_at?: number
  limit_window_seconds?: number
}

export type CodexRateLimitWindows = {
  plan_type?: string
  primary_window?: CodexRateLimitWindow
  secondary_window?: CodexRateLimitWindow
}

export type RateLimitSource = {
  plan_type?: string
  rate_limit?: CodexRateLimitWindows
}

export type SubscriptionUsageSnapshot = CodexRateLimitWindows & {
  limit_reached?: boolean
  updated_at?: number
}

export function clampPercent(value: unknown): number {
  const v = Number(value)
  return Number.isFinite(v) ? Math.max(0, Math.min(100, v)) : 0
}

export function normalizePlanType(value: unknown): string {
  if (value == null) {
    return ''
  }
  return String(value).trim().toLowerCase()
}

function classifyWindowByDuration(
  windowData?: CodexRateLimitWindow | null
): 'weekly' | 'fiveHour' | null {
  const seconds = Number(windowData?.limit_window_seconds)
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return null
  }
  return seconds >= 24 * 60 * 60 ? 'weekly' : 'fiveHour'
}

export function resolveRateLimitWindows(data: RateLimitSource | null): {
  fiveHourWindow: CodexRateLimitWindow | null
  weeklyWindow: CodexRateLimitWindow | null
} {
  const rateLimit = data?.rate_limit ?? {}
  const primary = rateLimit?.primary_window ?? null
  const secondary = rateLimit?.secondary_window ?? null
  const windows = [primary, secondary].filter(Boolean) as CodexRateLimitWindow[]
  const planType = normalizePlanType(data?.plan_type ?? rateLimit?.plan_type)

  let fiveHourWindow: CodexRateLimitWindow | null = null
  let weeklyWindow: CodexRateLimitWindow | null = null

  for (const w of windows) {
    const bucket = classifyWindowByDuration(w)
    if (bucket === 'fiveHour' && !fiveHourWindow) {
      fiveHourWindow = w
      continue
    }
    if (bucket === 'weekly' && !weeklyWindow) {
      weeklyWindow = w
    }
  }

  if (planType === 'free') {
    if (!weeklyWindow) {
      weeklyWindow = primary ?? secondary ?? null
    }
    return { fiveHourWindow: null, weeklyWindow }
  }

  if (!fiveHourWindow && !weeklyWindow) {
    return { fiveHourWindow: primary, weeklyWindow: secondary }
  }

  if (!fiveHourWindow) {
    fiveHourWindow = windows.find((w) => w !== weeklyWindow) ?? null
  }
  if (!weeklyWindow) {
    weeklyWindow = windows.find((w) => w !== fiveHourWindow) ?? null
  }

  return { fiveHourWindow, weeklyWindow }
}

export type UsageVariant = 'info' | 'warning' | 'danger'

export function windowLabel(windowData?: CodexRateLimitWindow | null): {
  percent: number
  variant: UsageVariant
} {
  const percent = clampPercent(windowData?.used_percent)
  let variant: UsageVariant = 'info'
  if (percent >= 95) {
    variant = 'danger'
  } else if (percent >= 80) {
    variant = 'warning'
  }
  return { percent, variant }
}

export function parseSubscriptionUsageSnapshot(
  otherInfo: string | null | undefined
): SubscriptionUsageSnapshot | null {
  if (!otherInfo) {
    return null
  }
  try {
    const parsed = JSON.parse(otherInfo)
    const snapshot = parsed?.subscription_usage
    if (snapshot && typeof snapshot === 'object') {
      return snapshot as SubscriptionUsageSnapshot
    }
  } catch {
    return null
  }
  return null
}

export function formatUnixSeconds(unixSeconds: unknown): string {
  const v = Number(unixSeconds)
  return Number.isFinite(v) && v > 0 ? formatTimestampToDate(v) : '-'
}

export function resetCountdownSeconds(
  windowData: CodexRateLimitWindow | null | undefined,
  nowMs: number
): number | null {
  const resetAt = Number(windowData?.reset_at)
  if (!Number.isFinite(resetAt) || resetAt <= 0) {
    return null
  }
  const remaining = Math.floor(resetAt - nowMs / 1000)
  return remaining > 0 ? remaining : null
}

export function formatDurationSeconds(
  seconds: unknown,
  t: (key: string) => string
): string {
  const s = Number(seconds)
  if (!Number.isFinite(s) || s <= 0) {
    return '-'
  }

  const total = Math.floor(s)
  const days = Math.floor(total / 86400)
  const hours = Math.floor((total % 86400) / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60

  if (days > 0) {
    return `${days}${t('d')} ${hours}${t('h')} ${minutes}${t('m')}`
  }
  if (hours > 0) {
    return `${hours}${t('h')} ${minutes}${t('m')}`
  }
  if (minutes > 0) {
    return `${minutes}${t('m')} ${secs}${t('s')}`
  }
  return `${secs}${t('s')}`
}

export type ClaudeLimitResetRequest = {
  program: 'cedar_ember' | 'juniper_tide'
  grant_id?: string
  resets_left?: number
}

export type ClaudeFullResetRow =
  | { available: false }
  | {
      available: true
      label: string
      resetsLeft: number
      resetsTotal: number
      endsAt: number
      clears: string[]
      cooldownUntil: number
      coolingDown: boolean
      canReset: boolean
      request: ClaudeLimitResetRequest
    }

export type ClaudeFiveHourResetRow =
  | { canReset: true; request: ClaudeLimitResetRequest }
  | {
      canReset: false
      reasonKind: 'not_at_wall' | 'client_identity' | 'none'
    }
  | { canReset: false; reasonKind: 'raw'; rawReason: string }
  | { canReset: false; reasonKind: 'next_available'; nextAvailableAt: number }

export type ClaudeLimitResets =
  | { state: 'client_version' }
  | { state: 'not_returned' }
  | {
      state: 'ready'
      full: ClaudeFullResetRow
      fiveHour: ClaudeFiveHourResetRow
    }

type ClaudeLimitResetSource = {
  limit_reset_unavailable?: string
  data?: unknown
}

type UnknownRecord = Record<string, unknown>

function asRecord(value: unknown): UnknownRecord | null {
  return value !== null && typeof value === 'object'
    ? (value as UnknownRecord)
    : null
}

export function parseIsoTimestamp(value: unknown): number {
  if (typeof value !== 'string' || value === '') {
    return 0
  }
  const ms = Date.parse(value)
  return Number.isFinite(ms) ? Math.floor(ms / 1000) : 0
}

function resolveFullResetRow(
  cedar: UnknownRecord,
  nowSeconds: number
): ClaudeFullResetRow {
  const grants = Array.isArray(cedar.grants) ? cedar.grants : []
  const grant = grants
    .map(asRecord)
    .find((g) => g !== null && g.id === cedar.next_grant_id)
  if (!grant || typeof grant.id !== 'string') {
    return { available: false }
  }

  const resetsLeft = Number(grant.resets_left) || 0
  const cooldownUntil = parseIsoTimestamp(cedar.cooldown_until)
  const hasCooldown =
    cedar.cooldown_until !== null &&
    cedar.cooldown_until !== undefined &&
    cedar.cooldown_until !== ''
  const coolingDown =
    hasCooldown && (cooldownUntil === 0 || cooldownUntil > nowSeconds)
  const canReset =
    cedar.eligible === true &&
    grant.usable_now === true &&
    grant.paused !== true &&
    resetsLeft > 0 &&
    (grant.use_requires_limit !== true || cedar.at_limit === true) &&
    !coolingDown

  return {
    available: true,
    label: String(grant.label ?? ''),
    resetsLeft,
    resetsTotal: Number(grant.resets_total) || 0,
    endsAt: parseIsoTimestamp(grant.ends_at),
    clears: Array.isArray(grant.clears) ? grant.clears.map(String) : [],
    cooldownUntil,
    coolingDown,
    canReset,
    request: {
      program: 'cedar_ember',
      grant_id: grant.id,
      resets_left: resetsLeft,
    },
  }
}

function resolveFiveHourResetRow(
  juniper: UnknownRecord | null
): ClaudeFiveHourResetRow {
  if (!juniper) {
    return { canReset: false, reasonKind: 'none' }
  }
  if (juniper.eligible === true && juniper.available === true) {
    return { canReset: true, request: { program: 'juniper_tide' } }
  }

  const reason = juniper.ineligible_reason
  if (reason === 'not_at_wall') {
    return { canReset: false, reasonKind: 'not_at_wall' }
  }
  if (reason === 'surface' || reason === 'cli_version') {
    return { canReset: false, reasonKind: 'client_identity' }
  }
  if (typeof reason === 'string' && reason !== '') {
    return { canReset: false, reasonKind: 'raw', rawReason: reason }
  }

  const nextAvailableAt = parseIsoTimestamp(juniper.next_available_at)
  if (nextAvailableAt > 0) {
    return { canReset: false, reasonKind: 'next_available', nextAvailableAt }
  }
  return { canReset: false, reasonKind: 'none' }
}

export function resolveClaudeLimitResets(
  response: ClaudeLimitResetSource,
  nowSeconds: number
): ClaudeLimitResets {
  if (response.limit_reset_unavailable === 'client_version') {
    return { state: 'client_version' }
  }

  const data = asRecord(response.data)
  const cedar = asRecord(data?.cedar_ember)
  const juniper = asRecord(data?.juniper_tide)
  if (!cedar && !juniper) {
    return { state: 'not_returned' }
  }

  return {
    state: 'ready',
    full: cedar ? resolveFullResetRow(cedar, nowSeconds) : { available: false },
    fiveHour: resolveFiveHourResetRow(juniper),
  }
}
