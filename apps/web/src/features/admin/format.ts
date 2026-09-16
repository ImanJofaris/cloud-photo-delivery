export { formatBytes } from "@/features/events/format"

const MINUTE_MS = 60_000

export function formatPendingAge(
  value: string | null | undefined,
  now: number = Date.now()
): string {
  if (!value) return "—"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "—"

  const diff = now - date.getTime()
  if (diff < MINUTE_MS) return "just now"

  const minutes = Math.floor(diff / MINUTE_MS)
  if (minutes < 60)
    return minutes === 1 ? "1 minute ago" : `${minutes} minutes ago`

  const hours = Math.floor(minutes / 60)
  if (hours < 48) return hours === 1 ? "1 hour ago" : `${hours} hours ago`

  const days = Math.floor(hours / 24)
  return days === 1 ? "1 day ago" : `${days} days ago`
}
