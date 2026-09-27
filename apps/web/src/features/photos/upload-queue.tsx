"use client"

import { RotateCcw, X } from "lucide-react"
import Link from "next/link"

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
          <div className="text-xs text-destructive" role="alert">
            <span>{item.error}</span>
            {item.errorCode === "PLAN_LIMIT_REACHED" && (
              <Link
                href="/billing"
                className="ml-1 font-medium underline underline-offset-4"
              >
                View plans
              </Link>
            )}
          </div>
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

export function visibleUploadItems(items: UploadItem[]): UploadItem[] {
  return items.filter((item) => item.status !== "ready")
}

export function batchProgress(items: UploadItem[]): {
  total: number
  ready: number
  percent: number
} {
  if (items.length === 0) return { total: 0, ready: 0, percent: 0 }
  const ready = items.filter((item) => item.status === "ready").length
  return {
    total: items.length,
    ready,
    percent: Math.round((ready / items.length) * 100),
  }
}

function UploadBatchProgress({ items }: { items: UploadItem[] }) {
  const { total, ready, percent } = batchProgress(items)
  return (
    <div className="space-y-1.5" data-testid="upload-batch-progress">
      <div className="flex items-baseline justify-between gap-3">
        <p className="text-sm font-medium">
          {ready} of {total} ready
        </p>
        <span className="text-xs text-muted-foreground tabular-nums">
          {percent}%
        </span>
      </div>
      <Progress value={percent} className="gap-0" />
    </div>
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
  const visible = visibleUploadItems(items)
  if (visible.length === 0) return null
  return (
    <div className="space-y-3">
      {items.length > 1 && <UploadBatchProgress items={items} />}
      <ul className="space-y-2" data-testid="upload-queue">
        {visible.map((item) => (
          <UploadQueueRow
            key={item.key}
            item={item}
            onRetry={onRetry}
            onCancel={onCancel}
            onDiscard={onDiscard}
          />
        ))}
      </ul>
    </div>
  )
}
