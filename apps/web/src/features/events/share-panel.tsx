"use client"

import { Copy, ExternalLink } from "lucide-react"
import * as React from "react"
import { toast } from "sonner"

import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

export function SharePanel({ slug }: { slug: string }) {
  const [copied, setCopied] = React.useState(false)

  async function copyLink() {
    const url = `${window.location.origin}/e/${slug}`
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
          Send this link to your client and their guests.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between gap-3 rounded-md bg-muted px-3 py-2">
          <code className="truncate text-sm">/e/{slug}</code>
          <Button size="sm" variant="ghost" onClick={() => void copyLink()}>
            <Copy />
            {copied ? "Copied" : "Copy link"}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          Guests open this link without an account. Password and private
          visibility are controlled in Settings; QR codes point here too.
        </p>
      </CardContent>
    </Card>
  )
}

export function GalleryLinkPreview({ slug }: { slug: string }) {
  return (
    <a
      className="inline-flex items-center gap-1 text-xs text-muted-foreground underline-offset-4 hover:underline"
      href={`/e/${slug}`}
      target="_blank"
      rel="noreferrer"
    >
      Open gallery
      <ExternalLink className="size-3" />
    </a>
  )
}
