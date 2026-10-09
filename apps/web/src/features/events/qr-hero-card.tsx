"use client"

import { Copy, Download, Printer } from "lucide-react"
import * as React from "react"
import { toast } from "sonner"

import { Button } from "@workspace/ui/components/button"
import { Card, CardContent } from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { formatCount } from "@/features/analytics/format"
import { useObjectUrl } from "@/lib/hooks/use-object-url"

import { useEventPublicUrl } from "./api"
import { qrDownloadFilename } from "./format"
import { buildPrintDocument, downloadBlob, useQrAssets } from "./qr-dialog"

export function QrHeroCard({
  eventId,
  eventName,
  scans,
}: {
  eventId: string
  eventName: string
  scans?: number
}) {
  const { assets, isError } = useQrAssets(eventId)
  const pngUrl = useObjectUrl(assets?.png ?? null)
  const publicUrl = useEventPublicUrl(eventId)
  const [copied, setCopied] = React.useState(false)

  async function copyLink() {
    if (!publicUrl.data?.url) return
    try {
      await navigator.clipboard.writeText(publicUrl.data.url)
      setCopied(true)
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
    <Card>
      <CardContent className="flex flex-col gap-5 sm:flex-row">
        <div className="flex size-[126px] shrink-0 items-center justify-center overflow-hidden rounded-xl bg-muted/40 p-2 ring-1 ring-border">
          {pngUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={pngUrl}
              alt={`QR code for ${eventName}`}
              width={126}
              height={126}
              decoding="async"
              className="size-full object-contain"
            />
          ) : isError ? (
            <p className="px-2 text-center text-xs text-destructive">
              Could not load the QR code.
            </p>
          ) : (
            <Skeleton className="size-full" />
          )}
        </div>

        <div className="flex min-w-0 flex-1 flex-col">
          <p className="font-medium">Gallery QR</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Print it on the booth standee or send it in chat.
            {typeof scans === "number" ? (
              <>
                {" "}
                <span className="font-medium text-foreground">
                  {formatCount(scans)} scans
                </span>{" "}
                so far for this event.
              </>
            ) : null}
          </p>
          <div className="mt-4 flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={!assets}
              onClick={handlePrint}
            >
              <Printer />
              Print standee
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={!pngUrl}
              onClick={() =>
                pngUrl &&
                downloadBlob(pngUrl, qrDownloadFilename(eventName, "png"))
              }
            >
              <Download />
              PNG
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={!publicUrl.data?.url}
              onClick={() => void copyLink()}
            >
              <Copy />
              {copied ? "Copied" : "Copy link"}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
