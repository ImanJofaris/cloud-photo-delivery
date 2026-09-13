"use client"

import { ImageOff } from "lucide-react"
import * as React from "react"

import { usePhotoUrl } from "../api"
import type { GalleryPhoto } from "../types"
import type { UrlVariant } from "../url-cache"

export function pickTileVariant(variants: string[]): UrlVariant | null {
  if (variants.includes("thumbnail")) return "thumbnail"
  if (variants.includes("medium")) return "medium"
  if (variants.includes("large")) return "large"
  return null
}

export function PhotoTile({
  slug,
  photo,
  index,
  eventName,
  onOpen,
}: {
  slug: string
  photo: GalleryPhoto
  index: number
  eventName: string
  onOpen: (index: number) => void
}) {
  const containerRef = React.useRef<HTMLButtonElement | null>(null)
  const retried = React.useRef(false)
  const [visible, setVisible] = React.useState(false)

  React.useEffect(() => {
    if (visible) return
    const node = containerRef.current
    if (!node) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) setVisible(true)
      },
      { rootMargin: "300px" }
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [visible])

  const variant = pickTileVariant(photo.variants)
  const url = usePhotoUrl(
    slug,
    photo.id,
    variant ?? "thumbnail",
    visible && variant !== null
  )

  const alt = `${eventName} photo ${index + 1}`
  const aspectRatio =
    photo.width && photo.height ? `${photo.width} / ${photo.height}` : "4 / 3"

  return (
    <button
      ref={containerRef}
      type="button"
      style={{ aspectRatio }}
      aria-label={`Open ${alt}`}
      className="group relative w-full overflow-hidden rounded-lg bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--gallery-primary)] focus-visible:ring-offset-2"
      onClick={() => onOpen(index)}
    >
      {url.data ? (
        // Signed URLs change per fetch, so the Next image optimizer would only
        // add latency. Plain img with lazy loading is intentional.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={url.data}
          alt={alt}
          loading="lazy"
          decoding="async"
          className="size-full object-cover transition-transform duration-200 group-hover:scale-[1.02] motion-reduce:transition-none motion-reduce:group-hover:scale-100"
          onError={() => {
            if (!retried.current) {
              retried.current = true
              void url.refetch()
            }
          }}
        />
      ) : (
        <span className="flex size-full items-center justify-center">
          <ImageOff
            className="size-5 text-muted-foreground"
            aria-hidden="true"
          />
        </span>
      )}
    </button>
  )
}
