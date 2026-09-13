"use client"

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query"

import type { components } from "@workspace/api-client"

import { apiCall } from "@/lib/auth/api"

import { billingKeys } from "./keys"

type PlanList = components["schemas"]["PlanList"]
type SubscriptionView = components["schemas"]["SubscriptionView"]
type SubscribeRequest = components["schemas"]["SubscribeRequest"]
type SubscribeResult = components["schemas"]["SubscribeResult"]
type PlanChangeRequest = components["schemas"]["PlanChangeRequest"]
type Subscription = components["schemas"]["Subscription"]
type InvoiceList = components["schemas"]["InvoiceList"]

function invalidateBilling(
  queryClient: ReturnType<typeof useQueryClient>
): void {
  void queryClient.invalidateQueries({ queryKey: billingKeys.subscription() })
  void queryClient.invalidateQueries({ queryKey: billingKeys.invoices() })
}

export function usePlans() {
  return useQuery({
    queryKey: billingKeys.plans(),
    queryFn: () => apiCall<PlanList>((client) => client.GET("/billing/plans")),
  })
}

export function useSubscription() {
  return useQuery({
    queryKey: billingKeys.subscription(),
    queryFn: () =>
      apiCall<SubscriptionView>((client) =>
        client.GET("/billing/subscription")
      ),
    staleTime: 0,
    refetchOnWindowFocus: true,
  })
}

export function useInvoices() {
  return useInfiniteQuery({
    queryKey: billingKeys.invoices(),
    queryFn: ({ pageParam }) =>
      apiCall<InvoiceList>((client) =>
        client.GET("/billing/invoices", {
          params: {
            query: pageParam ? { limit: 20, cursor: pageParam } : { limit: 20 },
          },
        })
      ),
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
  })
}

export function useSubscribe() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: SubscribeRequest) =>
      apiCall<SubscribeResult>((client) =>
        client.POST("/billing/subscribe", { body: input })
      ),
    onSuccess: () => invalidateBilling(queryClient),
  })
}

export function useUpgrade() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: PlanChangeRequest) =>
      apiCall<Subscription>((client) =>
        client.POST("/billing/upgrade", { body: input })
      ),
    onSuccess: () => invalidateBilling(queryClient),
  })
}

export function useDowngrade() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: PlanChangeRequest) =>
      apiCall<Subscription>((client) =>
        client.POST("/billing/downgrade", { body: input })
      ),
    onSuccess: () => invalidateBilling(queryClient),
  })
}

export function useCancelSubscription() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () =>
      apiCall<Subscription>((client) => client.POST("/billing/cancel")),
    onSuccess: () => invalidateBilling(queryClient),
  })
}

export function useResumeSubscription() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () =>
      apiCall<Subscription>((client) => client.POST("/billing/resume")),
    onSuccess: () => invalidateBilling(queryClient),
  })
}
