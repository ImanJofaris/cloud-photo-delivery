const dateFormatter = new Intl.DateTimeFormat("en-MY", {
  timeZone: "Asia/Kuala_Lumpur",
  dateStyle: "medium",
})

const dateTimeFormatter = new Intl.DateTimeFormat("en-MY", {
  timeZone: "Asia/Kuala_Lumpur",
  dateStyle: "medium",
  timeStyle: "short",
})

function parse(value: string | null | undefined) {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

export function formatDate(value: string | null | undefined): string {
  const date = parse(value)
  return date ? dateFormatter.format(date) : "—"
}

export function formatDateTime(value: string | null | undefined): string {
  const date = parse(value)
  return date ? dateTimeFormatter.format(date) : "—"
}

export function formatBytes(bytes: number | null | undefined): string {
  if (!bytes || bytes <= 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  const digits = unit === 0 || value >= 10 ? 0 : 1
  return `${value.toFixed(digits)} ${units[unit]}`
}

export const EXPIRY_WARN_DAYS = 7

const DAY_MS = 86_400_000

export type ExpiryTone = "neutral" | "warning" | "expired"

export function expiryFromNow(
  value: string | null | undefined,
  now: number = Date.now()
): { tone: ExpiryTone; label: string } {
  const date = parse(value)
  if (!date) return { tone: "neutral", label: "Never expires" }

  const diff = date.getTime() - now
  if (diff <= 0) {
    const days = Math.max(1, Math.ceil(-diff / DAY_MS))
    return {
      tone: "expired",
      label: `Expired ${days === 1 ? "1 day" : `${days} days`} ago`,
    }
  }

  if (diff < DAY_MS) return { tone: "warning", label: "Expires today" }

  const days = Math.ceil(diff / DAY_MS)
  const tone: ExpiryTone = days <= EXPIRY_WARN_DAYS ? "warning" : "neutral"
  return { tone, label: `Expires in ${days === 1 ? "1 day" : `${days} days`}` }
}

export function formatTimeUntil(
  value: string | null | undefined,
  now: number = Date.now()
): string | null {
  const date = parse(value)
  if (!date) return null

  const diff = date.getTime() - now
  if (diff <= 0) return null

  const minutes = Math.ceil(diff / 60_000)
  if (minutes < 60)
    return `${minutes === 1 ? "1 minute" : `${minutes} minutes`}`

  const hours = Math.ceil(minutes / 60)
  if (hours < 48) return `${hours === 1 ? "1 hour" : `${hours} hours`}`

  const days = Math.ceil(hours / 24)
  return days === 1 ? "1 day" : `${days} days`
}

export function qrDownloadFilename(
  eventName: string,
  extension: "png" | "svg"
): string {
  const base = eventName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
  return `${base || "event"}-qr.${extension}`
}
