"use client"

import { RotateCcw, X } from "lucide-react"

import { Button } from "@workspace/ui/components/button"
import { Progress } from "@workspace/ui/components/progress"

import { formatBytes } from "@/features/events/format"

import type { UploadItem } from "./uploader"

function statusText(item: UploadItem): string {
  switch (item.status) {
    case "queued":
      return "Waiting"
    case "uploading":
      return `${item.progress}%`
    case "processing":
      return "Processing"
    case "ready":
      return "Ready"
    default:
      return "Failed"
  }
}

function UploadQueueRow({
  item,
  onRetry,
  onCancel,
  onDiscard,
}: {
  item: UploadItem
  onRetry: (key: string) => void
  onCancel: (key: string) => void
  onDiscard: (key: string) => void
}) {
  const showProgress = item.status === "queued" || item.status === "uploading"
  return (
    <li className="flex items-center gap-3 rounded-md border px-3 py-2">
      <div className="min-w-0 flex-1 space-y-1.5">
        <div className="flex items-baseline justify-between gap-3">
          <p className="truncate text-sm">{item.filename}</p>
          <span className="shrink-0 text-xs text-muted-foreground">
            {formatBytes(item.size)} · {statusText(item)}
          </span>
        </div>
        {showProgress && <Progress value={item.progress} className="gap-0" />}
        {item.error && (
          <p className="text-xs text-destructive" role="alert">
            {item.error}
          </p>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-1">
        {item.status === "failed" && item.file && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => onRetry(item.key)}
          >
            <RotateCcw />
            Retry
          </Button>
        )}
        {(showProgress || item.status === "processing") && (
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label={`Cancel ${item.filename}`}
            onClick={() => onCancel(item.key)}
          >
            <X />
          </Button>
        )}
        {item.status !== "processing" && !showProgress && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => onDiscard(item.key)}
          >
            Remove
          </Button>
        )}
      </div>
    </li>
  )
}

export function UploadQueueList({
  items,
  onRetry,
  onCancel,
  onDiscard,
}: {
  items: UploadItem[]
  onRetry: (key: string) => void
  onCancel: (key: string) => void
  onDiscard: (key: string) => void
}) {
  if (items.length === 0) return null
  return (
    <ul className="space-y-2" data-testid="upload-queue">
      {items.map((item) => (
        <UploadQueueRow
          key={item.key}
          item={item}
          onRetry={onRetry}
          onCancel={onCancel}
          onDiscard={onDiscard}
        />
      ))}
    </ul>
  )
}
