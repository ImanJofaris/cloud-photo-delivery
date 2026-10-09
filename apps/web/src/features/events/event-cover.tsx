"use client"

import { ImageIcon } from "lucide-react"
import * as React from "react"

import { Skeleton } from "@workspace/ui/components/skeleton"
import { cn } from "@workspace/ui/lib/utils"

import { usePhotoThumbnail } from "@/features/photos/api"

export function EventCoverThumbnail({
  coverPhotoId,
  className,
}: {
  coverPhotoId?: string | null
  className?: string
}) {
  const containerRef = React.useRef<HTMLDivElement | null>(null)
  const [visible, setVisible] = React.useState(false)
  const [failedUrl, setFailedUrl] = React.useState<string | null>(null)

  React.useEffect(() => {
    if (visible || !coverPhotoId) return
    const node = containerRef.current
    if (!node) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) setVisible(true)
      },
      { rootMargin: "200px" }
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [visible, coverPhotoId])

  const thumbnail = usePhotoThumbnail(
    coverPhotoId ?? "",
    visible && Boolean(coverPhotoId)
  )
  const url = thumbnail.data?.url ?? null
  const showImage = url !== null && url !== failedUrl

  return (
    <div
      ref={containerRef}
      className={cn(
        "flex size-12 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-muted",
        className
      )}
    >
      {showImage ? (
        // Signed URLs change per fetch, so the Next image optimizer would only
        // add latency; plain img with lazy loading is intentional.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={url}
          alt=""
          loading="lazy"
          decoding="async"
          className="size-full object-cover"
          onError={() => setFailedUrl(url)}
        />
      ) : thumbnail.isPending && visible && coverPhotoId ? (
        <Skeleton className="size-full" />
      ) : (
        <ImageIcon
          className="size-5 text-muted-foreground"
          aria-hidden="true"
        />
      )}
    </div>
  )
}
