"use client"

import { Copy, Download, Printer, QrCode } from "lucide-react"
import * as React from "react"
import { toast } from "sonner"

import { Button } from "@workspace/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@workspace/ui/components/dialog"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { apiBlobCall } from "@/lib/auth/api"
import { useObjectUrl } from "@/lib/hooks/use-object-url"

import { useEventPublicUrl } from "./api"
import { qrDownloadFilename } from "./format"

export function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;")
}

export function buildPrintDocument({
  svg,
  eventName,
}: {
  svg: string
  eventName: string
}): string {
  return `<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<title>${escapeHtml(eventName)} QR code</title>
<style>
  body { font-family: system-ui, sans-serif; text-align: center; padding: 2rem; }
  h1 { font-size: 1.25rem; margin: 0 0 1.5rem; }
  svg { width: 320px; height: 320px; }
  p { margin-top: 1.5rem; color: #555; }
</style>
</head>
<body>
<h1>${escapeHtml(eventName)}</h1>
${svg}
<p>Scan to view photos</p>
<script>window.onload = function () { window.print() }</script>
</body>
</html>`
}

export function downloadBlob(url: string, filename: string) {
  const anchor = document.createElement("a")
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}

type QrAssets = {
  png: Blob
  svg: Blob
  svgText: string
}

function QrDialogBody({
  eventId,
  eventName,
}: {
  eventId: string
  eventName: string
}) {
  const [assets, setAssets] = React.useState<QrAssets | null>(null)
  const [isError, setIsError] = React.useState(false)
  const publicUrl = useEventPublicUrl(eventId)

  React.useEffect(() => {
    let cancelled = false

    async function load() {
      try {
        const [pngBlob, svgBlob] = await Promise.all([
          apiBlobCall((client) =>
            client.GET("/events/{eventID}/qr.png", {
              params: { path: { eventID: eventId } },
              query: { size: 1024 },
              parseAs: "blob",
            })
          ),
          apiBlobCall((client) =>
            client.GET("/events/{eventID}/qr.svg", {
              params: { path: { eventID: eventId } },
              parseAs: "blob",
            })
          ),
        ])
        const svgText = await svgBlob.text()
        if (!cancelled) setAssets({ png: pngBlob, svg: svgBlob, svgText })
      } catch {
        if (!cancelled) setIsError(true)
      }
    }

    void load()
    return () => {
      cancelled = true
    }
  }, [eventId])

  const pngUrl = useObjectUrl(assets?.png ?? null)
  const svgUrl = useObjectUrl(assets?.svg ?? null)

  async function copyLink() {
    if (!publicUrl.data?.url) return
    try {
      await navigator.clipboard.writeText(publicUrl.data.url)
      toast.success("Gallery link copied")
    } catch {
      toast.error("Could not copy the link")
    }
  }

  function handlePrint() {
    if (!assets) return
    const printWindow = window.open("", "_blank", "width=680,height=760")
    if (!printWindow) {
      toast.error("Allow pop-ups to print the QR code")
      return
    }
    printWindow.document.write(
      buildPrintDocument({ svg: assets.svgText, eventName })
    )
    printWindow.document.close()
  }

  return (
    <DialogContent className="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Event QR code</DialogTitle>
        <DialogDescription>
          Guests scan this to open {eventName}&apos;s gallery.
        </DialogDescription>
      </DialogHeader>

      <div className="flex min-h-56 items-center justify-center rounded-lg border bg-muted/40 p-4">
        {!assets && !isError && <Skeleton className="size-48" />}
        {isError && (
          <p className="text-sm text-destructive">
            Could not load the QR code. Please try again.
          </p>
        )}
        {pngUrl && (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={pngUrl}
            alt={`QR code for ${eventName}`}
            className="size-48"
            width={1024}
            height={1024}
            decoding="async"
          />
        )}
      </div>

      <div className="flex items-center gap-2 rounded-md bg-muted px-3 py-2">
        <code className="min-w-0 flex-1 truncate text-xs">
          {publicUrl.data?.url ?? "Loading link..."}
        </code>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          disabled={!publicUrl.data?.url}
          onClick={() => void copyLink()}
        >
          <Copy />
          Copy link
        </Button>
      </div>

      <DialogFooter className="sm:justify-start">
        <Button
          type="button"
          variant="outline"
          disabled={!pngUrl}
          onClick={() =>
            pngUrl && downloadBlob(pngUrl, qrDownloadFilename(eventName, "png"))
          }
        >
          <Download />
          Download PNG
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={!svgUrl}
          onClick={() =>
            svgUrl && downloadBlob(svgUrl, qrDownloadFilename(eventName, "svg"))
          }
        >
          <Download />
          Download SVG
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={!assets}
          onClick={handlePrint}
        >
          <Printer />
          Print
        </Button>
      </DialogFooter>
    </DialogContent>
  )
}

export function QrDialog({
  eventId,
  eventName,
  open,
  onOpenChange,
}: {
  eventId: string
  eventName: string
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open ? (
        <QrDialogBody eventId={eventId} eventName={eventName} />
      ) : null}
    </Dialog>
  )
}

export function QrTriggerButton({ onClick }: { onClick: () => void }) {
  return (
    <Button size="sm" variant="ghost" onClick={onClick}>
      <QrCode />
      QR code
    </Button>
  )
}
