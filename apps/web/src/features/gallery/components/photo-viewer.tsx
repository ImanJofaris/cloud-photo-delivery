"use client"

import { ChevronLeft, ChevronRight, X } from "lucide-react"
import * as React from "react"

import { Button } from "@workspace/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "@workspace/ui/components/dialog"

import { usePhotoUrl } from "../api"
import type { GalleryPhoto } from "../types"
import { DownloadButton } from "./download-button"

const SWIPE_THRESHOLD_PX = 40

export function PhotoViewer({
  slug,
  photos,
  index,
  eventName,
  allowDownload,
  allowOriginalDownload,
  hasNextPage,
  onLoadMore,
  onClose,
  onIndexChange,
}: {
  slug: string
  photos: GalleryPhoto[]
  index: number
  eventName: string
  allowDownload: boolean
  allowOriginalDownload: boolean
  hasNextPage: boolean
  onLoadMore: () => void
  onClose: () => void
  onIndexChange: (index: number) => void
}) {
  const photo = photos[index] ?? null
  const previous = index > 0 ? photos[index - 1] : null
  const next = index < photos.length - 1 ? photos[index + 1] : null

  usePhotoUrl(slug, previous?.id ?? "", "large", Boolean(previous))
  usePhotoUrl(slug, next?.id ?? "", "large", Boolean(next))
  const current = usePhotoUrl(slug, photo?.id ?? "", "large", Boolean(photo))

  const touchStartX = React.useRef<number | null>(null)

  const goPrevious = React.useCallback(() => {
    if (index > 0) onIndexChange(index - 1)
  }, [index, onIndexChange])

  const goNext = React.useCallback(() => {
    if (index < photos.length - 1) onIndexChange(index + 1)
    else if (hasNextPage) onLoadMore()
  }, [index, photos.length, hasNextPage, onIndexChange, onLoadMore])

  function handleKeyDown(event: React.KeyboardEvent) {
    if (event.key === "ArrowLeft") {
      event.preventDefault()
      goPrevious()
    } else if (event.key === "ArrowRight") {
      event.preventDefault()
      goNext()
    }
  }

  function handleTouchStart(event: React.TouchEvent) {
    touchStartX.current = event.touches[0]?.clientX ?? null
  }

  function handleTouchEnd(event: React.TouchEvent) {
    const start = touchStartX.current
    touchStartX.current = null
    const end = event.changedTouches[0]?.clientX
    if (start === null || end === undefined) return
    const delta = end - start
    if (Math.abs(delta) < SWIPE_THRESHOLD_PX) return
    if (delta < 0) goNext()
    else goPrevious()
  }

  const alt = `${eventName} photo ${index + 1}`

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
    >
      <DialogContent
        showCloseButton={false}
        aria-describedby={undefined}
        className="h-dvh w-screen max-w-none rounded-none bg-neutral-950 p-0 text-white ring-0 sm:max-w-none"
        onKeyDown={handleKeyDown}
        onTouchStart={handleTouchStart}
        onTouchEnd={handleTouchEnd}
      >
        <DialogTitle className="sr-only">
          {`Photo ${index + 1} of ${photos.length}`}
        </DialogTitle>

        <p className="absolute inset-x-0 top-[calc(env(safe-area-inset-top)+0.75rem)] text-center text-xs text-white/70">
          {index + 1} / {photos.length}
          {hasNextPage ? "+" : ""}
        </p>

        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="Close viewer"
          className="absolute right-3 top-[calc(env(safe-area-inset-top)+0.5rem)] z-10 text-white hover:bg-white/10 hover:text-white"
          onClick={onClose}
        >
          <X />
        </Button>

        <div className="flex size-full items-center justify-center px-2">
          {current.data ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={current.data}
              alt={alt}
              className="max-h-[calc(100dvh-8rem)] max-w-full object-contain"
            />
          ) : (
            <span
              role="status"
              aria-label="Loading photo"
              className="size-8 animate-spin rounded-full border-2 border-white/30 border-t-white motion-reduce:animate-none"
            />
          )}
        </div>

        <Button
          type="button"
          variant="ghost"
          size="icon-lg"
          aria-label="Previous photo"
          disabled={index === 0}
          className="absolute left-2 top-1/2 -translate-y-1/2 text-white hover:bg-white/10 hover:text-white disabled:opacity-30"
          onClick={goPrevious}
        >
          <ChevronLeft />
        </Button>

        <Button
          type="button"
          variant="ghost"
          size="icon-lg"
          aria-label="Next photo"
          disabled={index === photos.length - 1 && !hasNextPage}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-white hover:bg-white/10 hover:text-white disabled:opacity-30"
          onClick={goNext}
        >
          <ChevronRight />
        </Button>

        {allowDownload && photo ? (
          <div className="absolute inset-x-0 bottom-[calc(env(safe-area-inset-bottom)+1rem)] flex justify-center gap-2">
            <DownloadButton
              slug={slug}
              photoId={photo.id}
              variant="large"
              label="Download"
            />
            {allowOriginalDownload ? (
              <DownloadButton
                slug={slug}
                photoId={photo.id}
                variant="original"
                label="Original"
              />
            ) : null}
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
