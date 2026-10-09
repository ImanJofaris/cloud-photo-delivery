import { formatDate, formatDateTime } from "@/features/events/format"

export const UNKNOWN_EVENT = "Unknown event"

const MINUTE_MS = 60_000

export function formatRelativeTime(
  value: string | null,
  now: number = Date.now()
): string {
  if (!value) return "Never"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "Never"

  const diff = now - date.getTime()
  if (diff < MINUTE_MS) return "Just now"

  const minutes = Math.floor(diff / MINUTE_MS)
  if (minutes < 60)
    return minutes === 1 ? "1 min ago" : `${minutes} min ago`

  const hours = Math.floor(minutes / 60)
  if (hours < 48) return hours === 1 ? "1 hour ago" : `${hours} hours ago`

  return formatDate(value)
}

export const DEVICE_FRESH_WINDOW_MS = 24 * 60 * 60 * 1000

export type DeviceFreshness = "fresh" | "stale" | "never"

export function deviceFreshness(
  value: string | null,
  now: number = Date.now()
): DeviceFreshness {
  if (!value) return "never"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "never"
  return now - date.getTime() <= DEVICE_FRESH_WINDOW_MS ? "fresh" : "stale"
}

export function formatDeviceTimestamp(value: string | null): string {
  return value ? formatDateTime(value) : "Never"
}

export function resolveEventName(
  assignedEventId: string | null,
  names: Map<string, string>
): string | null {
  if (!assignedEventId) return null
  return names.get(assignedEventId) ?? UNKNOWN_EVENT
}
