import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { cn } from "@workspace/ui/lib/utils"

import { deviceFreshness, formatRelativeTime } from "./format"
import type { DeviceSummary } from "./summary"

const freshnessDot: Record<ReturnType<typeof deviceFreshness>, string> = {
  fresh: "bg-emerald-500",
  stale: "bg-amber-500",
  never: "bg-muted-foreground/40",
}

export function DeviceSummaryRow({ summary }: { summary: DeviceSummary }) {
  const lastUpload = summary.lastUpload
  const freshness = deviceFreshness(lastUpload?.lastUsedAt ?? null)
  const scopeLabel =
    summary.scopedEventCount === 0
      ? "No events assigned"
      : summary.scopedEventCount === 1
        ? "Across 1 event"
        : `Across ${summary.scopedEventCount} events`

  return (
    <div className="grid gap-4 sm:grid-cols-3">
      <Card>
        <CardHeader>
          <CardDescription>Active devices</CardDescription>
          <CardTitle className="text-2xl">{summary.activeCount}</CardTitle>
          <CardDescription className="text-xs">{scopeLabel}</CardDescription>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <CardDescription>Last upload</CardDescription>
          <CardTitle className="flex items-center gap-2 text-2xl">
            <span
              className={cn("size-2 rounded-full", freshnessDot[freshness])}
              aria-hidden="true"
            />
            {lastUpload ? formatRelativeTime(lastUpload.lastUsedAt) : "—"}
          </CardTitle>
          <CardDescription className="text-xs">
            {lastUpload ? lastUpload.deviceName : "No uploads yet"}
          </CardDescription>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <CardDescription>Revoked devices</CardDescription>
          <CardTitle className="text-2xl">{summary.revokedCount}</CardTitle>
          <CardDescription className="text-xs">Kept for audit</CardDescription>
        </CardHeader>
      </Card>
    </div>
  )
}
