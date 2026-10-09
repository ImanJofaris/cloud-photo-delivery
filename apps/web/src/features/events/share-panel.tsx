"use client"

import { Copy, ExternalLink, Globe, QrCode } from "lucide-react"
import * as React from "react"
import { toast } from "sonner"

import { Button, buttonVariants } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { cn } from "@workspace/ui/lib/utils"

import { useEventPublicUrl } from "./api"

export function OpenGalleryLink({
  slug,
  className,
}: {
  slug: string
  className?: string
}) {
  return (
    <a
      href={`/e/${slug}`}
      target="_blank"
      rel="noreferrer"
      className={cn(buttonVariants({ variant: "outline" }), className)}
    >
      Open gallery
      <ExternalLink aria-hidden="true" />
    </a>
  )
}

export function SharePanel({
  eventId,
  slug,
  onShowQr,
}: {
  eventId: string
  slug: string
  onShowQr: () => void
}) {
  const [copied, setCopied] = React.useState(false)
  const publicUrl = useEventPublicUrl(eventId)

  const fullUrl = publicUrl.data?.url ?? null
  const displayUrl = fullUrl
    ? fullUrl.replace(/^https?:\/\//, "")
    : `/e/${slug}`

  async function copyLink() {
    const url = fullUrl ?? `${window.location.origin}/e/${slug}`
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
      toast.success("Gallery link copied")
    } catch {
      toast.error("Could not copy the link")
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Share</CardTitle>
        <CardDescription>
          Send this link or QR code to your client and their guests.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center gap-2 rounded-control bg-muted px-3 py-2">
          <Globe
            className="size-3.5 shrink-0 text-muted-foreground"
            aria-hidden="true"
          />
          <span className="min-w-0 flex-1 truncate font-mono text-xs">
            {displayUrl}
          </span>
          <Button
            type="button"
            size="icon-xs"
            variant="ghost"
            aria-label="Copy gallery link"
            onClick={() => void copyLink()}
          >
            <Copy />
          </Button>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" onClick={onShowQr}>
            <QrCode />
            QR code
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => void copyLink()}
          >
            {copied ? "Copied" : "Copy link"}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          Guests open this link without an account. Password and private
          visibility are controlled in Settings.
        </p>
      </CardContent>
    </Card>
  )
}
