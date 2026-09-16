"use client"

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query"

import { ApiError, type components } from "@workspace/api-client"

import { apiCall } from "@/lib/auth/api"

import { createEventSchema, type EventSettingsValues } from "./schema"
import { eventKeys, type EventFilters } from "./keys"

type Event = components["schemas"]["Event"]
type EventList = components["schemas"]["EventList"]
type EventSettings = components["schemas"]["EventSettings"]
type EventDashboard = components["schemas"]["EventDashboard"]
type EventURL = components["schemas"]["EventURL"]
type EventWithSettings = components["schemas"]["EventWithSettings"]
type EventStatus = components["schemas"]["EventStatus"]
type CreateEventRequest = components["schemas"]["CreateEventRequest"]
type UpdateEventRequest = components["schemas"]["UpdateEventRequest"]
type UpdateEventSettingsRequest =
  components["schemas"]["UpdateEventSettingsRequest"]
type Export = components["schemas"]["Export"]
type ExportStatus = components["schemas"]["ExportStatus"]
type ExtendEventRequest = components["schemas"]["ExtendEventRequest"]

export const EXPORT_POLL_INTERVAL_MS = 5_000

export function exportRefetchInterval(
  status: ExportStatus | undefined
): number | false {
  return status === "pending" || status === "processing"
    ? EXPORT_POLL_INTERVAL_MS
    : false
}

export function isNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404
}

export function useEvents(filters: EventFilters) {
  return useInfiniteQuery({
    queryKey: eventKeys.list(filters),
    queryFn: ({ pageParam }) => {
      const query: {
        limit: number
        q?: string
        status?: EventStatus
        cursor?: string
      } = { limit: 20 }

      if (filters.q) query.q = filters.q
      if (filters.status !== "all") query.status = filters.status
      if (pageParam) query.cursor = pageParam

      return apiCall<EventList>((client) =>
        client.GET("/events", { params: { query } })
      )
    },
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
  })
}

export function useEvent(eventId: string) {
  return useQuery({
    queryKey: eventKeys.detail(eventId),
    queryFn: () =>
      apiCall<Event>((client) =>
        client.GET("/events/{eventID}", {
          params: { path: { eventID: eventId } },
        })
      ),
  })
}

export function useEventSettings(eventId: string) {
  return useQuery({
    queryKey: eventKeys.settings(eventId),
    queryFn: () =>
      apiCall<EventSettings>((client) =>
        client.GET("/events/{eventID}/settings", {
          params: { path: { eventID: eventId } },
        })
      ),
  })
}

export function useEventPublicUrl(eventId: string, enabled = true) {
  return useQuery({
    queryKey: eventKeys.url(eventId),
    queryFn: () =>
      apiCall<EventURL>((client) =>
        client.GET("/events/{eventID}/url", {
          params: { path: { eventID: eventId } },
        })
      ),
    enabled,
  })
}

export function useEventOptions() {
  return useQuery({
    queryKey: eventKeys.options(),
    queryFn: async () => {
      const items: Event[] = []
      let cursor: string | undefined
      for (let page = 0; page < 10; page += 1) {
        const result = await apiCall<EventList>((client) =>
          client.GET("/events", { params: { query: { limit: 100, cursor } } })
        )
        items.push(...result.items)
        if (!result.nextCursor) break
        cursor = result.nextCursor
      }
      return items
    },
  })
}

export function useEventDashboard(eventId: string) {
  return useQuery({
    queryKey: eventKeys.dashboard(eventId),
    queryFn: () =>
      apiCall<EventDashboard>((client) =>
        client.GET("/events/{eventID}/dashboard", {
          params: { path: { eventID: eventId } },
        })
      ),
  })
}

export function useCreateEvent() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: CreateEventRequest) =>
      apiCall<EventWithSettings>((client) =>
        client.POST("/events", { body: input })
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: eventKeys.lists() })
    },
  })
}

export function useUpdateEvent(eventId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: UpdateEventRequest) =>
      apiCall<Event>((client) =>
        client.PATCH("/events/{eventID}", {
          params: { path: { eventID: eventId } },
          body: input,
        })
      ),
    onSuccess: (event) => {
      queryClient.setQueryData(eventKeys.detail(eventId), event)
      void queryClient.invalidateQueries({ queryKey: eventKeys.lists() })
    },
  })
}

export function useUpdateEventSettings(eventId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: UpdateEventSettingsRequest) =>
      apiCall<EventSettings>((client) =>
        client.PATCH("/events/{eventID}/settings", {
          params: { path: { eventID: eventId } },
          body: input,
        })
      ),
    onSuccess: (settings) => {
      queryClient.setQueryData(eventKeys.settings(eventId), settings)
    },
  })
}

export function useArchiveEvent(eventId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () =>
      apiCall<Event>((client) =>
        client.POST("/events/{eventID}/archive", {
          params: { path: { eventID: eventId } },
        })
      ),
    onSuccess: (event) => {
      queryClient.setQueryData(eventKeys.detail(eventId), event)
      void queryClient.invalidateQueries({ queryKey: eventKeys.lists() })
    },
  })
}

export function useDeleteEvent(eventId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () =>
      apiCall<void>((client) =>
        client.DELETE("/events/{eventID}", {
          params: { path: { eventID: eventId } },
        })
      ),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: eventKeys.detail(eventId) })
      void queryClient.invalidateQueries({ queryKey: eventKeys.lists() })
    },
  })
}

export function useEventExport(eventId: string, exportId: string | null) {
  return useQuery({
    queryKey: eventKeys.export(eventId, exportId ?? ""),
    queryFn: () =>
      apiCall<Export>((client) =>
        client.GET("/events/{eventID}/exports/{exportID}", {
          params: { path: { eventID: eventId, exportID: exportId ?? "" } },
        })
      ),
    enabled: exportId !== null,
    refetchInterval: (query) => exportRefetchInterval(query.state.data?.status),
  })
}

export function useCreateEventExport(eventId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () =>
      apiCall<Export>((client) =>
        client.POST("/events/{eventID}/exports", {
          params: { path: { eventID: eventId } },
        })
      ),
    onSuccess: (exportJob) => {
      queryClient.setQueryData(
        eventKeys.export(eventId, exportJob.id),
        exportJob
      )
    },
  })
}

export function useExtendEvent(eventId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (days: number) => {
      const body: ExtendEventRequest = { days }
      return apiCall<Event>((client) =>
        client.POST("/events/{eventID}/extend", {
          params: { path: { eventID: eventId } },
          body,
        })
      )
    },
    onSuccess: (event) => {
      queryClient.setQueryData(eventKeys.detail(eventId), event)
      void queryClient.invalidateQueries({ queryKey: eventKeys.lists() })
    },
  })
}

export function toCreateEventRequest(values: {
  name: string
  eventDate?: string
  clientName?: string
  clientEmail?: string
  location?: string
  description?: string
}): CreateEventRequest {
  const parsed = createEventSchema.parse(values)

  return {
    name: parsed.name,
    ...(parsed.eventDate ? { eventDate: parsed.eventDate } : {}),
    ...(parsed.clientName ? { clientName: parsed.clientName } : {}),
    ...(parsed.clientEmail ? { clientEmail: parsed.clientEmail } : {}),
    ...(parsed.location ? { location: parsed.location } : {}),
    ...(parsed.description ? { description: parsed.description } : {}),
  }
}

export function toUpdateEventSettingsRequest(
  values: EventSettingsValues
): UpdateEventSettingsRequest {
  return {
    visibility: values.visibility,
    ...(values.password ? { password: values.password } : {}),
    allowDownload: values.allowDownload,
    allowOriginalDownload: values.allowOriginalDownload,
    watermarkEnabled: values.watermarkEnabled,
  }
}
