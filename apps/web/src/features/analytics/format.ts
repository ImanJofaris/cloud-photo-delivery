import type { components } from "@workspace/api-client"

type AnalyticsDay = components["schemas"]["AnalyticsDay"]

export type AnalyticsMetric =
  "galleryViews" | "uniqueVisitors" | "downloads" | "qrScans"

const DAY_MS = 86_400_000

const numberFormatter = new Intl.NumberFormat("en-MY")
const dayFormatter = new Intl.DateTimeFormat("en-MY", {
  timeZone: "UTC",
  month: "short",
  day: "numeric",
})
const dayLongFormatter = new Intl.DateTimeFormat("en-MY", {
  timeZone: "UTC",
  year: "numeric",
  month: "short",
  day: "numeric",
})

function parseUtcDay(date: string): Date | null {
  const parsed = new Date(`${date}T00:00:00Z`)
  return Number.isNaN(parsed.getTime()) ? null : parsed
}

export function formatCount(value: number | null | undefined): string {
  return typeof value === "number" ? numberFormatter.format(value) : "—"
}

export function formatUtcDay(date: string): string {
  const parsed = parseUtcDay(date)
  return parsed ? dayFormatter.format(parsed) : "—"
}

export function formatUtcDayLong(date: string): string {
  const parsed = parseUtcDay(date)
  return parsed ? dayLongFormatter.format(parsed) : "—"
}

export function utcDaySeries(days: number, now: Date = new Date()): string[] {
  const total = Math.max(0, Math.floor(days))
  const today = Date.UTC(
    now.getUTCFullYear(),
    now.getUTCMonth(),
    now.getUTCDate()
  )
  const series: string[] = []
  for (let offset = total - 1; offset >= 0; offset -= 1) {
    series.push(new Date(today - offset * DAY_MS).toISOString().slice(0, 10))
  }
  return series
}

export function fillDailySeries(
  daily: AnalyticsDay[],
  days: number,
  now: Date = new Date()
): AnalyticsDay[] {
  const byDate = new Map(daily.map((day) => [day.date, day]))
  return utcDaySeries(days, now).map(
    (date) =>
      byDate.get(date) ?? {
        date,
        galleryViews: 0,
        uniqueVisitors: 0,
        downloads: 0,
        qrScans: 0,
      }
  )
}

export function seriesTotal(
  daily: AnalyticsDay[],
  metric: AnalyticsMetric
): number {
  return daily.reduce((sum, day) => sum + day[metric], 0)
}
