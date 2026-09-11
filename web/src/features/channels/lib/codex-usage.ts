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

export type CodexUsageSnapshot = CodexRateLimitWindows & {
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

export type CodexUsageVariant = 'info' | 'warning' | 'danger'

export function windowLabel(windowData?: CodexRateLimitWindow | null): {
  percent: number
  variant: CodexUsageVariant
} {
  const percent = clampPercent(windowData?.used_percent)
  let variant: CodexUsageVariant = 'info'
  if (percent >= 95) {
    variant = 'danger'
  } else if (percent >= 80) {
    variant = 'warning'
  }
  return { percent, variant }
}

export function parseCodexUsageSnapshot(
  otherInfo: string | null | undefined
): CodexUsageSnapshot | null {
  if (!otherInfo) {
    return null
  }
  try {
    const parsed = JSON.parse(otherInfo)
    const snapshot = parsed?.codex_usage
    if (snapshot && typeof snapshot === 'object') {
      return snapshot as CodexUsageSnapshot
    }
  } catch {
    return null
  }
  return null
}
