"use client"

import { ImageOff } from "lucide-react"

import type { components } from "@workspace/api-client"
import { Button } from "@workspace/ui/components/button"
import { Card, CardContent } from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { isNotFound } from "./errors"
import { useEventPhotos } from "./api"
import { PhotoTile } from "./photo-tile"

type Photo = components["schemas"]["Photo"]

export function PhotoGrid({
  eventId,
  onDelete,
}: {
  eventId: string
  onDelete: (photo: Photo) => void
}) {
  const photos = useEventPhotos(eventId)
  const items = photos.data?.pages.flatMap((page) => page.items) ?? []

  if (photos.isPending) {
    return (
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {Array.from({ length: 8 }).map((_, index) => (
          <Skeleton
            key={index}
            className="aspect-square rounded-md sm:aspect-[4/3]"
          />
        ))}
      </div>
    )
  }

  if (photos.isError) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center gap-3 py-10 text-center">
          <p className="text-sm text-muted-foreground">
            {isNotFound(photos.error)
              ? "This event no longer exists."
              : "Could not load photos. Please try again."}
          </p>
          <Button variant="outline" onClick={() => void photos.refetch()}>
            Try again
          </Button>
        </CardContent>
      </Card>
    )
  }

  if (items.length === 0) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center gap-2 py-10 text-center">
          <ImageOff className="size-6 text-muted-foreground" />
          <p className="text-sm font-medium">No photos yet</p>
          <p className="text-xs text-muted-foreground">
            Upload files above; they appear here as they upload and process.
          </p>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {items.map((photo) => (
          <PhotoTile key={photo.id} photo={photo} onDelete={onDelete} />
        ))}
      </div>
      {photos.hasNextPage && (
        <div className="flex justify-center">
          <Button
            variant="outline"
            onClick={() => void photos.fetchNextPage()}
            disabled={photos.isFetchingNextPage}
          >
            {photos.isFetchingNextPage ? "Loading..." : "Load more"}
          </Button>
        </div>
      )}
    </div>
  )
}
