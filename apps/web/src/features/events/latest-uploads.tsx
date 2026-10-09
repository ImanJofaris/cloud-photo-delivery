"use client"

import { ChevronRight, ImageIcon } from "lucide-react"
import * as React from "react"

import type { components } from "@workspace/api-client"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { formatCount } from "@/features/analytics/format"
import { useEventPhotos, usePhotoThumbnail } from "@/features/photos/api"

type Photo = components["schemas"]["Photo"]

const MAX_THUMBS = 6

function UploadThumb({
  photo,
  onOpen,
}: {
  photo: Photo
  onOpen: () => void
}) {
  const hasThumbnail =
    photo.status === "READY" && photo.variants.includes("thumbnail")
  const thumbnail = usePhotoThumbnail(photo.id, hasThumbnail)

  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label={`View all photos (${photo.filename || "latest upload"})`}
      className="relative size-20 shrink-0 overflow-hidden rounded-xl bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      {thumbnail.data ? (
        // Signed URLs change per fetch, so the Next image optimizer would only
        // add latency; plain img with lazy loading is intentional.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={thumbnail.data.url}
          alt={photo.filename || "Event photo"}
          loading="lazy"
          decoding="async"
          className="size-full object-cover"
        />
      ) : (
        <span className="flex size-full items-center justify-center">
          <ImageIcon
            className="size-5 text-muted-foreground"
            aria-hidden="true"
          />
        </span>
      )}
    </button>
  )
}

export function LatestUploads({
  eventId,
  total,
  onViewAll,
}: {
  eventId: string
  total?: number
  onViewAll: () => void
}) {
  const photos = useEventPhotos(eventId)
  const items =
    photos.data?.pages.flatMap((page) => page.items).slice(0, MAX_THUMBS) ?? []
  const knownTotal = typeof total === "number" ? total : undefined
  const remaining =
    knownTotal !== undefined ? Math.max(0, knownTotal - items.length) : 0

  return (
    <Card className="gap-3">
      <CardHeader className="items-center">
        <CardTitle>Latest uploads</CardTitle>
        <CardAction>
          <Button variant="link" size="sm" onClick={onViewAll}>
            View all{knownTotal !== undefined ? ` ${formatCount(knownTotal)}` : ""}
            <ChevronRight />
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        {photos.isPending ? (
          <div className="flex gap-2">
            {Array.from({ length: MAX_THUMBS }).map((_, index) => (
              <Skeleton key={index} className="size-20 shrink-0 rounded-xl" />
            ))}
          </div>
        ) : photos.isError ? (
          <p className="text-sm text-muted-foreground">
            Could not load photos. Open the Photos tab to try again.
          </p>
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No photos yet. Uploads appear here as they process.
          </p>
        ) : (
          <div className="flex gap-2 overflow-hidden">
            {items.map((photo) => (
              <UploadThumb key={photo.id} photo={photo} onOpen={onViewAll} />
            ))}
            {remaining > 0 && (
              <button
                type="button"
                onClick={onViewAll}
                aria-label={`View all photos (${remaining} more)`}
                className="flex size-20 shrink-0 items-center justify-center rounded-xl bg-muted text-sm font-medium text-muted-foreground transition-colors hover:bg-muted/70 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              >
                +{formatCount(remaining)}
              </button>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
