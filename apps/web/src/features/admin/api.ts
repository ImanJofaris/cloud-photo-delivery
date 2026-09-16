"use client"

import { useInfiniteQuery, useQuery } from "@tanstack/react-query"

import type { components } from "@workspace/api-client"

import { apiCall } from "@/lib/auth/api"

import { adminKeys } from "./keys"

type AdminStats = components["schemas"]["AdminStats"]
type AdminUserList = components["schemas"]["AdminUserList"]
type AdminSubscriptionList = components["schemas"]["AdminSubscriptionList"]
type AdminHealth = components["schemas"]["AdminHealth"]

const PAGE_SIZE = 20
const STALE_TIME = 60_000

export function useAdminStats(enabled = true) {
  return useQuery({
    queryKey: adminKeys.stats(),
    queryFn: () => apiCall<AdminStats>((client) => client.GET("/admin/stats")),
    staleTime: STALE_TIME,
    enabled,
  })
}

export function useAdminUsers(enabled = true) {
  return useInfiniteQuery({
    queryKey: adminKeys.users(),
    queryFn: ({ pageParam }) =>
      apiCall<AdminUserList>((client) =>
        client.GET("/admin/users", {
          params: {
            query: pageParam
              ? { limit: PAGE_SIZE, cursor: pageParam }
              : { limit: PAGE_SIZE },
          },
        })
      ),
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    staleTime: STALE_TIME,
    enabled,
  })
}

export function useAdminSubscriptions(enabled = true) {
  return useInfiniteQuery({
    queryKey: adminKeys.subscriptions(),
    queryFn: ({ pageParam }) =>
      apiCall<AdminSubscriptionList>((client) =>
        client.GET("/admin/subscriptions", {
          params: {
            query: pageParam
              ? { limit: PAGE_SIZE, cursor: pageParam }
              : { limit: PAGE_SIZE },
          },
        })
      ),
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    staleTime: STALE_TIME,
    enabled,
  })
}

export function useAdminHealth(enabled = true) {
  return useQuery({
    queryKey: adminKeys.health(),
    queryFn: () => apiCall<AdminHealth>((client) => client.GET("/admin/health")),
    staleTime: STALE_TIME,
    enabled,
  })
}
