"use client"

import { Check, ImageOff, Star, Trash2 } from "lucide-react"

import { Button } from "@workspace/ui/components/button"
import { cn } from "@workspace/ui/lib/utils"

import { usePhotoThumbnail } from "./api"
import { PhotoStatusBadge } from "./photo-status-badge"
import type { components } from "@workspace/api-client"

type Photo = components["schemas"]["Photo"]

export function PhotoTile({
  photo,
  isCover,
  selecting,
  selected,
  onDelete,
  onToggleSelect,
}: {
  photo: Photo
  isCover: boolean
  selecting: boolean
  selected: boolean
  onDelete: (photo: Photo) => void
  onToggleSelect: (photo: Photo) => void
}) {
  const hasThumbnail =
    photo.status === "READY" && photo.variants.includes("thumbnail")
  const thumbnail = usePhotoThumbnail(photo.id, hasThumbnail)

  return (
    <figure className="group space-y-1.5">
      <div
        className={cn(
          "relative aspect-square overflow-hidden rounded-xl border bg-muted",
          selected && "ring-2 ring-primary"
        )}
      >
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
          className="absolute bottom-1.5 left-1.5"
        />
        {isCover && (
          <span
            role="img"
            aria-label="Cover photo"
            className="absolute left-1.5 top-1.5 flex size-5 items-center justify-center rounded-full bg-amber-100 text-amber-700 ring-2 ring-background"
          >
            <Star className="size-3 fill-current" />
          </span>
        )}
        {selecting ? (
          <button
            type="button"
            aria-pressed={selected}
            aria-label={`Select ${photo.filename || "photo"}`}
            className="absolute inset-0 flex items-start justify-end outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
            onClick={() => onToggleSelect(photo)}
          >
            <span
              className={cn(
                "m-1.5 flex size-5 items-center justify-center rounded-full border-2 border-background shadow-sm",
                selected
                  ? "bg-primary text-primary-foreground"
                  : "bg-background/80 text-transparent"
              )}
            >
              <Check className="size-3" />
            </span>
          </button>
        ) : (
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
        )}
      </div>
      <figcaption className="truncate text-xs text-muted-foreground">
        {photo.filename || "Untitled"}
      </figcaption>
    </figure>
  )
}
