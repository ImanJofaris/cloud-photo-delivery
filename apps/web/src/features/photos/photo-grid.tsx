"use client"

import { Check, ImageOff, Trash2 } from "lucide-react"
import * as React from "react"

import type { components } from "@workspace/api-client"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select"
import { Skeleton } from "@workspace/ui/components/skeleton"
import { cn } from "@workspace/ui/lib/utils"

import { useEventPhotos } from "./api"
import { isNotFound } from "./errors"
import type { PhotoStatusFilter } from "./keys"
import { PhotoTile } from "./photo-tile"

type Photo = components["schemas"]["Photo"]

export type PhotoSortOrder = "newest" | "oldest"

const STATUS_FILTERS: { value: PhotoStatusFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "PROCESSING", label: "Processing" },
  { value: "FAILED", label: "Failed" },
]

export function sortPhotos(
  photos: Photo[],
  order: PhotoSortOrder
): Photo[] {
  const direction = order === "newest" ? -1 : 1
  return [...photos].sort(
    (a, b) => direction * (Date.parse(a.createdAt) - Date.parse(b.createdAt))
  )
}

export function PhotoGrid({
  eventId,
  coverPhotoId = null,
  photoCount,
  onDelete,
}: {
  eventId: string
  coverPhotoId?: string | null
  photoCount?: number
  onDelete: (photos: Photo[]) => void
}) {
  const [status, setStatus] = React.useState<PhotoStatusFilter>("all")
  const [sort, setSort] = React.useState<PhotoSortOrder>("newest")
  const [selecting, setSelecting] = React.useState(false)
  const [selectedIds, setSelectedIds] = React.useState<ReadonlySet<string>>(
    () => new Set()
  )

  const photos = useEventPhotos(eventId, status)
  const items = React.useMemo(
    () => photos.data?.pages.flatMap((page) => page.items) ?? [],
    [photos.data]
  )

  const sortedItems = React.useMemo(
    () => sortPhotos(items, sort),
    [items, sort]
  )
  const selectedPhotos = sortedItems.filter((photo) =>
    selectedIds.has(photo.id)
  )
  const activeFilter =
    STATUS_FILTERS.find((filter) => filter.value === status) ?? STATUS_FILTERS[0]

  function toggleSelecting() {
    setSelecting((current) => !current)
    setSelectedIds(new Set())
  }

  function togglePhoto(photo: Photo) {
    setSelectedIds((current) => {
      const next = new Set(current)
      if (next.has(photo.id)) next.delete(photo.id)
      else next.add(photo.id)
      return next
    })
  }

  const title =
    status === "all"
      ? photoCount != null
        ? `${photoCount} ${photoCount === 1 ? "photo" : "photos"}`
        : `${items.length} ${items.length === 1 ? "photo" : "photos"}`
      : `${items.length}${photos.hasNextPage ? "+" : ""} ${activeFilter.label.toLowerCase()} ${
          items.length === 1 && !photos.hasNextPage ? "photo" : "photos"
        }`

  return (
    <Card className="gap-0 py-0">
      <CardHeader className="flex flex-wrap items-center justify-between gap-x-3 gap-y-3 px-4 pt-4 pb-3">
        <CardTitle>{title}</CardTitle>
        <div className="flex flex-wrap items-center gap-2">
          <div
            role="group"
            aria-label="Filter photos by status"
            className="flex flex-wrap gap-1"
          >
            {STATUS_FILTERS.map((filter) => {
              const active = filter.value === status
              return (
                <Button
                  key={filter.value}
                  type="button"
                  size="sm"
                  variant="outline"
                  aria-pressed={active}
                  className={cn(
                    "rounded-full px-3",
                    active &&
                      "border-transparent bg-foreground text-background hover:bg-foreground/90"
                  )}
                  onClick={() => setStatus(filter.value)}
                >
                  {filter.label}
                </Button>
              )
            })}
          </div>
          <Select
            value={sort}
            onValueChange={(value) => setSort(value as PhotoSortOrder)}
          >
            <SelectTrigger
              size="sm"
              aria-label="Sort photos"
              className="w-32"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="newest">Newest</SelectItem>
              <SelectItem value="oldest">Oldest</SelectItem>
            </SelectContent>
          </Select>
          <Button
            type="button"
            variant="outline"
            size="sm"
            aria-pressed={selecting}
            className={cn(
              selecting &&
                "border-transparent bg-foreground text-background hover:bg-foreground/90"
            )}
            onClick={toggleSelecting}
          >
            <Check />
            Select
          </Button>
        </div>
      </CardHeader>
      <CardContent className="px-4 pb-4">
        {photos.isPending ? (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8">
            {Array.from({ length: 8 }).map((_, index) => (
              <Skeleton key={index} className="aspect-square rounded-xl" />
            ))}
          </div>
        ) : photos.isError ? (
          <div className="flex flex-col items-center gap-3 py-10 text-center">
            <p className="text-sm text-muted-foreground">
              {isNotFound(photos.error)
                ? "This event no longer exists."
                : "Could not load photos. Please try again."}
            </p>
            <Button variant="outline" onClick={() => void photos.refetch()}>
              Try again
            </Button>
          </div>
        ) : sortedItems.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-10 text-center">
            <ImageOff className="size-6 text-muted-foreground" />
            <p className="text-sm font-medium">
              {status === "all"
                ? "No photos yet"
                : `No ${activeFilter.label.toLowerCase()} photos`}
            </p>
            <p className="text-xs text-muted-foreground">
              {status === "all"
                ? "Use the drop zone above; they appear here as they upload and process."
                : "Try another filter."}
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8">
              {sortedItems.map((photo) => (
                <PhotoTile
                  key={photo.id}
                  photo={photo}
                  isCover={photo.id === coverPhotoId}
                  selecting={selecting}
                  selected={selectedIds.has(photo.id)}
                  onDelete={(target) => onDelete([target])}
                  onToggleSelect={togglePhoto}
                />
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
        )}
      </CardContent>
      {!photos.isPending && !photos.isError && sortedItems.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-2 px-4 pb-4">
          {selecting ? (
            <div className="inline-flex items-center gap-2 rounded-full bg-foreground py-1.5 pr-2 pl-3.5 text-xs font-semibold text-background">
              <span aria-live="polite">{selectedPhotos.length} selected</span>
              <Button
                type="button"
                size="xs"
                variant="secondary"
                disabled={selectedPhotos.length === 0}
                onClick={() => onDelete(selectedPhotos)}
              >
                <Trash2 />
                Delete
              </Button>
              <Button
                type="button"
                size="xs"
                variant="ghost"
                disabled={selectedPhotos.length === 0}
                className="text-background hover:bg-background/15 hover:text-background"
                onClick={() => setSelectedIds(new Set())}
              >
                Clear
              </Button>
            </div>
          ) : (
            <span className="text-xs text-muted-foreground">
              Cover photo marked with a star. Selection disables single-photo
              actions.
            </span>
          )}
        </div>
      )}
    </Card>
  )
}
