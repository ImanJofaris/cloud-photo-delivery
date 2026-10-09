"use client"

import { RotateCcw, X } from "lucide-react"
import Link from "next/link"

import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Progress } from "@workspace/ui/components/progress"
import { cn } from "@workspace/ui/lib/utils"

import { formatBytes } from "@/features/events/format"

import type { UploadItem } from "./uploader"

type RowTone = "default" | "success" | "danger"

const toneIndicator: Record<RowTone, string> = {
  default: "",
  success: "[&_[data-slot=progress-indicator]]:bg-emerald-600",
  danger: "[&_[data-slot=progress-indicator]]:bg-destructive",
}

function rowMeta(item: UploadItem): { text: string; tone: RowTone } {
  switch (item.status) {
    case "queued":
      return { text: "Waiting", tone: "default" }
    case "uploading":
      return { text: `${item.progress}%`, tone: "default" }
    case "processing":
      return { text: "Processing", tone: "success" }
    case "ready":
      return { text: "Done", tone: "success" }
    case "interrupted":
      return { text: "Interrupted", tone: "danger" }
    default:
      return { text: "Failed", tone: "danger" }
  }
}

function isActive(item: UploadItem): boolean {
  return (
    item.status === "queued" ||
    item.status === "uploading" ||
    item.status === "processing"
  )
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
  const meta = rowMeta(item)
  const progress =
    item.status === "ready" || item.status === "processing"
      ? 100
      : item.status === "queued"
        ? 0
        : item.progress

  return (
    <li className="flex items-center gap-3 border-t py-3 first:border-t-0 first:pt-0 last:pb-0">
      <div className="min-w-0 flex-1 space-y-1.5">
        <div className="flex items-baseline justify-between gap-3">
          <p
            className={cn(
              "truncate text-sm font-medium",
              meta.tone === "danger" && "text-destructive"
            )}
          >
            {item.filename}
          </p>
          <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
            {formatBytes(item.size)} · {meta.text}
          </span>
        </div>
        <Progress
          value={progress}
          className={cn("gap-0", toneIndicator[meta.tone])}
        />
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
            size="xs"
            variant="outline"
            onClick={() => onRetry(item.key)}
          >
            <RotateCcw />
            Retry
          </Button>
        )}
        {isActive(item) && (
          <Button
            type="button"
            size="icon-xs"
            variant="ghost"
            aria-label={`Cancel ${item.filename}`}
            onClick={() => onCancel(item.key)}
          >
            <X />
          </Button>
        )}
        {(item.status === "failed" || item.status === "interrupted") && (
          <Button
            type="button"
            size="xs"
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

export function UploadQueueCard({
  items,
  onRetry,
  onCancel,
  onDiscard,
  onClearFinished,
}: {
  items: UploadItem[]
  onRetry: (key: string) => void
  onCancel: (key: string) => void
  onDiscard: (key: string) => void
  onClearFinished: () => void
}) {
  if (items.length === 0) return null
  const { total, ready } = batchProgress(items)

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          Uploading · {ready} of {total}
        </CardTitle>
        <CardAction>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={ready === 0}
            onClick={onClearFinished}
          >
            Clear finished
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        <ul data-testid="upload-queue">
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
      </CardContent>
    </Card>
  )
}
