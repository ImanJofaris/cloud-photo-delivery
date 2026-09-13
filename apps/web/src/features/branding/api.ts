"use client"

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import type { components } from "@workspace/api-client"

import { apiCall } from "@/lib/auth/api"

import { brandingKeys } from "./keys"

type Branding = components["schemas"]["Branding"]
type BrandingAsset = components["schemas"]["BrandingAsset"]
type BrandingAssetRequest = components["schemas"]["BrandingAssetRequest"]
type BrandingUpdateRequest = components["schemas"]["BrandingUpdateRequest"]

export function useBranding() {
  return useQuery({
    queryKey: brandingKeys.detail(),
    queryFn: () => apiCall<Branding>((client) => client.GET("/account/branding")),
  })
}

export function useUpdateBranding() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: BrandingUpdateRequest) =>
      apiCall<Branding>((client) =>
        client.PATCH("/account/branding", { body: input })
      ),
    onSuccess: (branding) => {
      queryClient.setQueryData(brandingKeys.detail(), branding)
    },
  })
}

export function useCreateBrandingAssetUpload() {
  return useMutation({
    mutationFn: (input: BrandingAssetRequest) =>
      apiCall<BrandingAsset>((client) =>
        client.POST("/account/branding/assets", { body: input })
      ),
  })
}
