import { formatDateTime } from "@/features/events/format"

export const UNKNOWN_EVENT = "Unknown event"

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
