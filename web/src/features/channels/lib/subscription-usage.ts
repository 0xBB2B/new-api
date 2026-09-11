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
  reset_after_seconds?: number
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

export function formatDurationSeconds(
  seconds: unknown,
  t: (key: string) => string
): string {
  const s = Number(seconds)
  if (!Number.isFinite(s) || s <= 0) {
    return '-'
  }

  const total = Math.floor(s)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60

  if (hours > 0) {
    return `${hours}${t('h')} ${minutes}${t('m')}`
  }
  if (minutes > 0) {
    return `${minutes}${t('m')} ${secs}${t('s')}`
  }
  return `${secs}${t('s')}`
}
