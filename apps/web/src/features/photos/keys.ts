import type { components } from "@workspace/api-client"

export type PhotoStatus = components["schemas"]["PhotoStatus"]
export type PhotoStatusFilter = PhotoStatus | "all"

export const photoKeys = {
  all: ["photos"] as const,
  lists: () => [...photoKeys.all, "list"] as const,
  list: (eventId: string, status: PhotoStatusFilter) =>
    [...photoKeys.lists(), eventId, status] as const,
  url: (photoId: string, variant: string) =>
    [...photoKeys.all, "url", photoId, variant] as const,
}
