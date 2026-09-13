"use client"

import { ImageOff, Trash2 } from "lucide-react"

import { Button } from "@workspace/ui/components/button"

import { usePhotoThumbnail } from "./api"
import { PhotoStatusBadge } from "./photo-status-badge"
import type { components } from "@workspace/api-client"

type Photo = components["schemas"]["Photo"]

export function PhotoTile({
  photo,
  onDelete,
}: {
  photo: Photo
  onDelete: (photo: Photo) => void
}) {
  const hasThumbnail =
    photo.status === "READY" && photo.variants.includes("thumbnail")
  const thumbnail = usePhotoThumbnail(photo.id, hasThumbnail)

  return (
    <figure className="group space-y-1.5">
      <div className="relative aspect-square overflow-hidden rounded-md border bg-muted sm:aspect-[4/3]">
        {thumbnail.data ? (
          // Signed URLs change per fetch, so the Next image optimizer would
          // only add latency; plain img with lazy loading is intentional.
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={thumbnail.data.url}
            alt={photo.filename || "Event photo"}
            loading="lazy"
            decoding="async"
            className="size-full object-cover"
            onError={() => {
              if (!thumbnail.isFetching) void thumbnail.refetch()
            }}
          />
        ) : (
          <div className="flex size-full items-center justify-center">
            <ImageOff className="size-5 text-muted-foreground" />
          </div>
        )}
        <PhotoStatusBadge
          status={photo.status}
          className="absolute left-1.5 top-1.5"
        />
        <Button
          type="button"
          variant="secondary"
          size="icon-sm"
          aria-label={`Delete ${photo.filename || "photo"}`}
          className="absolute right-1.5 top-1.5 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
          onClick={() => onDelete(photo)}
        >
          <Trash2 />
        </Button>
      </div>
      <figcaption className="truncate text-xs text-muted-foreground">
        {photo.filename || "Untitled"}
      </figcaption>
    </figure>
  )
}
