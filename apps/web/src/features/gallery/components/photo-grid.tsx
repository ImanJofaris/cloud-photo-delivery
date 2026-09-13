"use client"

import * as React from "react"

import { Button } from "@workspace/ui/components/button"

import type { GalleryPhoto } from "../types"
import { PhotoTile } from "./photo-tile"

export function PhotoGrid({
  slug,
  photos,
  eventName,
  hasNextPage,
  isFetchingNextPage,
  onLoadMore,
  onOpen,
}: {
  slug: string
  photos: GalleryPhoto[]
  eventName: string
  hasNextPage: boolean
  isFetchingNextPage: boolean
  onLoadMore: () => void
  onOpen: (index: number) => void
}) {
  const sentinelRef = React.useRef<HTMLDivElement | null>(null)

  React.useEffect(() => {
    const node = sentinelRef.current
    if (!node || !hasNextPage) return
    if (typeof IntersectionObserver === "undefined") return
    const observer = new IntersectionObserver(
      (entries) => {
        if (
          entries.some((entry) => entry.isIntersecting) &&
          !isFetchingNextPage
        ) {
          onLoadMore()
        }
      },
      { rootMargin: "600px" }
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [hasNextPage, isFetchingNextPage, onLoadMore])

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
        {photos.map((photo, index) => (
          <PhotoTile
            key={photo.id}
            slug={slug}
            photo={photo}
            index={index}
            eventName={eventName}
            onOpen={onOpen}
          />
        ))}
      </div>

      {hasNextPage ? (
        <div ref={sentinelRef} className="flex justify-center">
          <Button
            type="button"
            variant="outline"
            disabled={isFetchingNextPage}
            onClick={onLoadMore}
          >
            {isFetchingNextPage ? "Loading..." : "Load more"}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
