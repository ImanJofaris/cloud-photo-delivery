"use client"

import { useQuery } from "@tanstack/react-query"

import type { components } from "@workspace/api-client"

import { apiCall } from "@/lib/auth/api"

import { accountKeys } from "./keys"

type Profile = components["schemas"]["Profile"]

export function useProfile() {
  return useQuery({
    queryKey: accountKeys.profile(),
    queryFn: () => apiCall<Profile>((client) => client.GET("/account/me")),
  })
}
