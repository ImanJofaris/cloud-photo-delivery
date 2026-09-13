"use client"

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type InfiniteData,
} from "@tanstack/react-query"

import { type components } from "@workspace/api-client"

import { eventKeys } from "@/features/events/keys"
import { apiCall } from "@/lib/auth/api"

import { photoKeys, type PhotoStatusFilter } from "./keys"
import { createLocalStorageRegistry } from "./storage"
import { putWithProgress } from "./transport"
import type { UploadDeps } from "./uploader"

type PhotoList = components["schemas"]["PhotoList"]
type PhotoStatus = components["schemas"]["PhotoStatus"]
type SignedURL = components["schemas"]["SignedURL"]
type InitializeUpload = components["schemas"]["InitializeUpload"]
type UploadStatus = components["schemas"]["UploadStatus"]
type PresignedUpload = components["schemas"]["PresignedUpload"]
type UploadPartURLs = components["schemas"]["UploadPartURLs"]
type UploadContentType =
  components["schemas"]["InitializeUploadRequest"]["contentType"]

export function hasPendingPhotos(
  data: InfiniteData<PhotoList> | undefined
): boolean {
  return Boolean(
    data?.pages.some((page) =>
      page.items.some(
        (photo) =>
          photo.status === "UPLOADING" || photo.status === "PROCESSING"
      )
    )
  )
}

export function useEventPhotos(
  eventId: string,
  status: PhotoStatusFilter = "all"
) {
  return useInfiniteQuery({
    queryKey: photoKeys.list(eventId, status),
    queryFn: ({ pageParam }) => {
      const query: { limit: number; cursor?: string; status?: PhotoStatus } = {
        limit: 60,
      }
      if (pageParam) query.cursor = pageParam
      if (status !== "all") query.status = status
      return apiCall<PhotoList>((client) =>
        client.GET("/events/{eventID}/photos", {
          params: { path: { eventID: eventId }, query },
        })
      )
    },
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    refetchInterval: (query) =>
      hasPendingPhotos(query.state.data) ? 4000 : false,
  })
}

function withoutPhoto(
  data: InfiniteData<PhotoList> | undefined,
  photoId: string
): InfiniteData<PhotoList> | undefined {
  if (!data) return data
  return {
    ...data,
    pages: data.pages.map((page) => ({
      ...page,
      items: page.items.filter((photo) => photo.id !== photoId),
    })),
  }
}

export function useDeletePhoto(eventId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (photoId: string) =>
      apiCall<void>((client) =>
        client.DELETE("/photos/{photoID}", {
          params: { path: { photoID: photoId } },
        })
      ),
    onMutate: async (photoId) => {
      await queryClient.cancelQueries({ queryKey: photoKeys.lists() })
      const snapshots = queryClient.getQueriesData<
        InfiniteData<PhotoList>
      >({ queryKey: photoKeys.lists() })
      queryClient.setQueriesData<InfiniteData<PhotoList>>(
        { queryKey: photoKeys.lists() },
        (data) => withoutPhoto(data, photoId)
      )
      return { snapshots }
    },
    onError: (_error, _photoId, context) => {
      for (const [key, data] of context?.snapshots ?? []) {
        queryClient.setQueryData(key, data)
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: photoKeys.lists() })
      void queryClient.invalidateQueries({
        queryKey: eventKeys.dashboard(eventId),
      })
    },
  })
}

export function usePhotoThumbnail(photoId: string, enabled: boolean) {
  return useQuery({
    queryKey: photoKeys.url(photoId, "thumbnail"),
    queryFn: () =>
      apiCall<SignedURL>((client) =>
        client.GET("/photos/{photoID}/url", {
          params: {
            path: { photoID: photoId },
            query: { variant: "thumbnail" },
          },
        })
      ),
    enabled,
    staleTime: 4 * 60 * 1000,
    gcTime: 5 * 60 * 1000,
    retry: 1,
  })
}

export function createUploadDeps(eventId: string): UploadDeps {
  return {
    initialize: (file, idempotencyKey) =>
      apiCall<InitializeUpload>((client) =>
        client.POST("/events/{eventID}/uploads", {
          params: {
            path: { eventID: eventId },
            header: { "Idempotency-Key": idempotencyKey },
          },
          body: {
            filename: file.name,
            contentType: file.type as UploadContentType,
            size: file.size,
          },
        })
      ),
    rePresign: (photoId) =>
      apiCall<PresignedUpload>((client) =>
        client.POST("/uploads/{photoID}/url", {
          params: { path: { photoID: photoId } },
        })
      ),
    uploadStatus: (photoId) =>
      apiCall<UploadStatus>((client) =>
        client.GET("/uploads/{photoID}", {
          params: { path: { photoID: photoId } },
        })
      ),
    partUrls: (photoId, partNumbers) =>
      apiCall<UploadPartURLs>((client) =>
        client.POST("/uploads/{photoID}/parts", {
          params: { path: { photoID: photoId } },
          body: { partNumbers },
        })
      ),
    completeMultipart: (photoId, parts) =>
      apiCall<void>((client) =>
        client.POST("/uploads/{photoID}/multipart/complete", {
          params: { path: { photoID: photoId } },
          body: { parts },
        })
      ),
    abortMultipart: (photoId) =>
      apiCall<void>((client) =>
        client.POST("/uploads/{photoID}/multipart/abort", {
          params: { path: { photoID: photoId } },
        })
      ),
    complete: (photoId, idempotencyKey) =>
      apiCall<UploadStatus>((client) =>
        client.POST("/uploads/{photoID}/complete", {
          params: {
            path: { photoID: photoId },
            header: { "Idempotency-Key": idempotencyKey },
          },
        })
      ),
    deletePhoto: (photoId) =>
      apiCall<void>((client) =>
        client.DELETE("/photos/{photoID}", {
          params: { path: { photoID: photoId } },
        })
      ),
    put: putWithProgress,
    registry: createLocalStorageRegistry(),
    randomUUID: () => crypto.randomUUID(),
  }
}
