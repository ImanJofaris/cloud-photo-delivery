"use client"

import * as React from "react"

import { Button } from "@workspace/ui/components/button"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { usePublicPhotos, useUnlockGallery } from "../api"
import { isUnauthorized } from "../errors"
import { toGalleryPhoto, type GalleryEvent } from "../types"
import { readUnlockToken, subscribeUnlock } from "../unlock"
import { GalleryEmpty } from "./gallery-empty"
import { GalleryHeader } from "./gallery-header"
import { PhotoGrid } from "./photo-grid"
import { PhotoViewer } from "./photo-viewer"
import { UnlockGate } from "./unlock-gate"

const DEFAULT_PRIMARY_COLOR = "#64748b"

export function GalleryShell({
  slug,
  event,
}: {
  slug: string
  event: GalleryEvent
}) {
  const [unlockDismissed, setUnlockDismissed] = React.useState(false)
  const [viewerIndex, setViewerIndex] = React.useState<number | null>(null)

  const hasStoredToken = React.useSyncExternalStore(
    subscribeUnlock,
    () => readUnlockToken(slug) !== null,
    () => false
  )

  const baseLocked =
    !unlockDismissed && Boolean(event.requiresUnlock) && !hasStoredToken
  const photosQuery = usePublicPhotos(slug, !baseLocked)
  const unlock = useUnlockGallery(slug)

  const locked = baseLocked || isUnauthorized(photosQuery.error)

  const photos = React.useMemo(() => {
    const items =
      photosQuery.data?.pages.flatMap((page) => page.photos?.items ?? []) ?? []
    return items.flatMap((photo) => {
      const normalized = photo ? toGalleryPhoto(photo) : null
      return normalized ? [normalized] : []
    })
  }, [photosQuery.data])

  async function handleUnlock(password: string) {
    try {
      await unlock.mutateAsync(password)
      setUnlockDismissed(true)
    } catch {
      // The gate renders the mapped error.
    }
  }

  const loadMore = React.useCallback(() => {
    void photosQuery.fetchNextPage()
  }, [photosQuery])

  const showGrid = !locked && !photosQuery.isPending && !photosQuery.isError
  const showEmpty = showGrid && photos.length === 0
  const showPhotosError =
    !locked && photosQuery.isError && !isUnauthorized(photosQuery.error)

  return (
    <div
      className="min-h-dvh bg-background"
      style={
        {
          "--gallery-primary":
            event.branding?.primaryColor ?? DEFAULT_PRIMARY_COLOR,
        } as React.CSSProperties
      }
    >
      <GalleryHeader slug={slug} event={event} viewable={!locked} />

      <main className="mx-auto w-full max-w-5xl px-3 pb-[calc(env(safe-area-inset-bottom)+4rem)] sm:px-4">
        {locked ? (
          <UnlockGate
            eventName={event.name}
            isPending={unlock.isPending}
            error={unlock.error}
            onSubmit={(password) => void handleUnlock(password)}
          />
        ) : showEmpty ? (
          <GalleryEmpty eventName={event.name} />
        ) : showPhotosError ? (
          <div className="flex flex-col items-center gap-3 py-16 text-center">
            <p className="text-sm text-muted-foreground">
              Could not load photos. Please try again.
            </p>
            <Button variant="outline" onClick={() => void photosQuery.refetch()}>
              Try again
            </Button>
          </div>
        ) : showGrid ? (
          <PhotoGrid
            slug={slug}
            photos={photos}
            eventName={event.name}
            hasNextPage={Boolean(photosQuery.hasNextPage)}
            isFetchingNextPage={photosQuery.isFetchingNextPage}
            onLoadMore={loadMore}
            onOpen={setViewerIndex}
          />
        ) : (
          <PhotoGridSkeleton />
        )}
      </main>

      {viewerIndex !== null && !locked && photos[viewerIndex] ? (
        <PhotoViewer
          slug={slug}
          photos={photos}
          index={viewerIndex}
          eventName={event.name}
          allowDownload={event.allowDownload}
          allowOriginalDownload={event.allowOriginalDownload}
          hasNextPage={Boolean(photosQuery.hasNextPage)}
          onLoadMore={loadMore}
          onClose={() => setViewerIndex(null)}
          onIndexChange={setViewerIndex}
        />
      ) : null}
    </div>
  )
}

function PhotoGridSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
      {Array.from({ length: 8 }).map((_, index) => (
        <Skeleton key={index} className="aspect-square w-full" />
      ))}
    </div>
  )
}
