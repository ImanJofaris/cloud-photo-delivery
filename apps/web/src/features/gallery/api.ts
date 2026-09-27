"use client"

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query"

import {
  ApiError,
  createApiClient,
  unwrapEnvelope,
  type components,
} from "@workspace/api-client"

import { apiVersionedBaseUrl } from "@/lib/env"

import { createBatchedUrlFetcher } from "./batch-url-fetcher"
import { galleryKeys } from "./keys"
import { clearUnlockToken, readUnlockToken, storeUnlockToken } from "./unlock"
import { createSignedUrlCache, type UrlVariant } from "./url-cache"

type PublicPhotoList = components["schemas"]["PublicPhotoList"]
type SignedURL = components["schemas"]["SignedURL"]
type BatchPhotoURL = components["schemas"]["BatchPhotoURL"]
type UnlockResult = components["schemas"]["UnlockResult"]

type Client = ReturnType<typeof createApiClient>
type ClientResult = { data?: unknown; error?: unknown; response: Response }

export const PHOTO_PAGE_SIZE = 50

const URL_RETRY_LIMIT = 4
const URL_RETRY_MAX_DELAY_MS = 60_000

function retryUrls(failureCount: number, error: unknown) {
  if (error instanceof ApiError && error.status === 429) {
    return failureCount < URL_RETRY_LIMIT
  }
  return failureCount < 1
}

function retryUrlsDelay(failureCount: number, error: unknown) {
  if (error instanceof ApiError && error.retryAfterSeconds !== null) {
    return Math.min(error.retryAfterSeconds * 1000, URL_RETRY_MAX_DELAY_MS)
  }
  return Math.min(500 * 2 ** failureCount, 8_000)
}

const clients = new Map<string, Client>()

function publicClient(slug: string): Client {
  let client = clients.get(slug)
  if (!client) {
    client = createApiClient({ baseUrl: apiVersionedBaseUrl() })
    client.use({
      onRequest({ request }) {
        const token = readUnlockToken(slug)
        if (token) {
          request.headers.set("X-Gallery-Unlock", token)
        }
        return request
      },
    })
    clients.set(slug, client)
  }
  return client
}

async function publicCall<T>(
  slug: string,
  call: (client: Client) => Promise<ClientResult>
): Promise<T> {
  try {
    return unwrapEnvelope<T>(await call(publicClient(slug)))
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      clearUnlockToken(slug)
      clearPhotoUrls(slug)
    }
    throw error
  }
}

const urlCaches = new Map<string, ReturnType<typeof createSignedUrlCache>>()

function urlCache(slug: string) {
  let cache = urlCaches.get(slug)
  if (!cache) {
    cache = createSignedUrlCache(
      createBatchedUrlFetcher(async (variant, photoIds) => {
        const batch = await publicCall<BatchPhotoURL>(slug, (client) =>
          client.POST("/public/events/{slug}/photos/urls", {
            params: { path: { slug } },
            body: { variant, photoIds },
          })
        )
        const urls: Record<string, SignedURL> = {}
        for (const [photoId, url] of Object.entries(batch.urls ?? {})) {
          urls[photoId] = { url, expiresIn: batch.expiresIn }
        }
        return urls
      })
    )
    urlCaches.set(slug, cache)
  }
  return cache
}

export function fetchPhotoUrl(
  slug: string,
  photoId: string,
  variant: UrlVariant,
  options?: { force?: boolean }
): Promise<string> {
  return urlCache(slug).fetch(photoId, variant, options)
}

export function clearPhotoUrls(slug: string) {
  urlCaches.get(slug)?.clear()
}

export function usePublicPhotos(slug: string, enabled: boolean) {
  return useInfiniteQuery({
    queryKey: galleryKeys.photos(slug),
    queryFn: ({ pageParam }) => {
      const query: { limit: number; cursor?: string } = {
        limit: PHOTO_PAGE_SIZE,
      }
      if (pageParam) query.cursor = pageParam
      return publicCall<PublicPhotoList>(slug, (client) =>
        client.GET("/public/events/{slug}/photos", {
          params: { path: { slug }, query },
        })
      )
    },
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.photos?.nextCursor ?? undefined,
    enabled,
  })
}

export function useUnlockGallery(slug: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (password: string) =>
      publicCall<UnlockResult>(slug, (client) =>
        client.POST("/public/events/{slug}/unlock", {
          params: { path: { slug } },
          body: { password },
        })
      ),
    onSuccess: (result) => {
      if (result.token && result.expiresIn) {
        storeUnlockToken(slug, result.token, result.expiresIn)
      }
      queryClient.removeQueries({ queryKey: galleryKeys.photos(slug) })
    },
  })
}

export function usePhotoUrl(
  slug: string,
  photoId: string,
  variant: UrlVariant,
  enabled: boolean
) {
  return useQuery({
    queryKey: galleryKeys.url(slug, photoId, variant),
    queryFn: () => fetchPhotoUrl(slug, photoId, variant),
    enabled,
    staleTime: 4 * 60 * 1000,
    gcTime: 5 * 60 * 1000,
    retry: retryUrls,
    retryDelay: retryUrlsDelay,
  })
}

export function usePhotoUrlWithFallback(
  slug: string,
  photoId: string,
  variants: UrlVariant[],
  enabled: boolean
) {
  return useQuery({
    queryKey: galleryKeys.url(slug, photoId, variants.join("|")),
    queryFn: async () => {
      let lastError: unknown = null
      for (const variant of variants) {
        try {
          return await fetchPhotoUrl(slug, photoId, variant)
        } catch (error) {
          if (
            error instanceof ApiError &&
            error.code === "VARIANT_UNAVAILABLE"
          ) {
            lastError = error
            continue
          }
          throw error
        }
      }
      throw lastError ?? new ApiError("VARIANT_UNAVAILABLE", "No variant", 404)
    },
    enabled,
    staleTime: 4 * 60 * 1000,
    gcTime: 5 * 60 * 1000,
    retry: retryUrls,
    retryDelay: retryUrlsDelay,
  })
}
