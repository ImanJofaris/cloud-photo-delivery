import type { components } from "@workspace/api-client"

type EventStatus = components["schemas"]["EventStatus"]

export type EventFilters = {
  q: string
  status: EventStatus | "all"
}

export const eventKeys = {
  all: ["events"] as const,
  lists: () => [...eventKeys.all, "list"] as const,
  list: (filters: EventFilters) => [...eventKeys.lists(), filters] as const,
  options: () => [...eventKeys.all, "options"] as const,
  detail: (id: string) => [...eventKeys.all, "detail", id] as const,
  settings: (id: string) => [...eventKeys.all, "settings", id] as const,
  dashboard: (id: string) => [...eventKeys.all, "dashboard", id] as const,
  url: (id: string) => [...eventKeys.all, "url", id] as const,
  export: (eventId: string, exportId: string) =>
    [...eventKeys.all, "export", eventId, exportId] as const,
}
