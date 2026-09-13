"use client"

import * as React from "react"
import { Download } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@workspace/ui/components/button"

import { fetchPhotoUrl } from "../api"
import type { UrlVariant } from "../url-cache"

export function DownloadButton({
  slug,
  photoId,
  variant,
  label,
  className,
}: {
  slug: string
  photoId: string
  variant: UrlVariant
  label: string
  className?: string
}) {
  const [busy, setBusy] = React.useState(false)

  async function download() {
    setBusy(true)
    try {
      const url = await fetchPhotoUrl(slug, photoId, variant)
      const response = await fetch(url)
      if (!response.ok) throw new Error("download failed")
      const blob = await response.blob()
      const objectUrl = URL.createObjectURL(blob)
      const anchor = document.createElement("a")
      anchor.href = objectUrl
      anchor.download = `photo-${photoId.slice(0, 8)}`
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(objectUrl)
    } catch {
      toast.error("Download failed. Please try again.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Button
      type="button"
      variant="secondary"
      size="sm"
      className={className}
      disabled={busy}
      onClick={() => void download()}
    >
      <Download />
      {label}
    </Button>
  )
}
