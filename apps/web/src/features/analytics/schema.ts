import { z } from "zod"

export const ANALYTICS_WINDOWS = [7, 30, 90, 365] as const
export const ANALYTICS_MIN_DAYS = 1
export const ANALYTICS_MAX_DAYS = 365
export const DEFAULT_ANALYTICS_WINDOW = 30

export const analyticsDaysSchema = z
  .number()
  .int()
  .min(ANALYTICS_MIN_DAYS)
  .max(ANALYTICS_MAX_DAYS)

export function parseAnalyticsDays(value: unknown): number {
  const candidate = typeof value === "string" ? Number(value) : value
  const parsed = analyticsDaysSchema.safeParse(candidate)
  return parsed.success ? parsed.data : DEFAULT_ANALYTICS_WINDOW
}
