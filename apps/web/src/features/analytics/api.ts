"use client"

import { useQuery } from "@tanstack/react-query"

import { type components } from "@workspace/api-client"

import { apiCall } from "@/lib/auth/api"

import { analyticsKeys } from "./keys"

type AccountAnalytics = components["schemas"]["AccountAnalytics"]
type EventAnalytics = components["schemas"]["EventAnalytics"]

export function useAccountAnalytics(days: number) {
  return useQuery({
    queryKey: analyticsKeys.account(days),
    queryFn: () =>
      apiCall<AccountAnalytics>((client) =>
        client.GET("/account/analytics", { params: { query: { days } } })
      ),
  })
}

export function useEventAnalytics(eventId: string, days: number) {
  return useQuery({
    queryKey: analyticsKeys.event(eventId, days),
    queryFn: () =>
      apiCall<EventAnalytics>((client) =>
        client.GET("/events/{eventID}/analytics", {
          params: { path: { eventID: eventId }, query: { days } },
        })
      ),
  })
}
