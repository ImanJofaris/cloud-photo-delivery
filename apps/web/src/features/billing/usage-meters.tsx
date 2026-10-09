"use client"

import type { components } from "@workspace/api-client"

import { Progress } from "@workspace/ui/components/progress"
import { cn } from "@workspace/ui/lib/utils"

import { formatBytes } from "@/features/events/format"

import { formatByteLimit, formatLimit, usagePercent } from "./format"

type Plan = components["schemas"]["Plan"]
type SubscriptionUsage = components["schemas"]["SubscriptionUsage"]

function Meter({
  label,
  used,
  limit,
  valueText,
  hint,
}: {
  label: string
  used: number
  limit: number
  valueText: string
  hint: string
}) {
  const unlimited = limit === 0
  const over = !unlimited && used > limit
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="text-muted-foreground">{label}</span>
        <span
          className={cn(
            "font-semibold tabular-nums",
            over && "text-destructive"
          )}
        >
          {valueText}
        </span>
      </div>
      {!unlimited && (
        <Progress
          value={usagePercent(used, limit)}
          className={cn(
            "[&_[data-slot=progress-track]]:h-1.5",
            over && "[&_[data-slot=progress-indicator]]:bg-destructive"
          )}
        />
      )}
      {over ? (
        <p className="text-xs text-destructive">
          Over the plan limit. Upgrade to unblock new actions.
        </p>
      ) : (
        <p className="text-xs text-muted-foreground">{hint}</p>
      )}
    </div>
  )
}

export function UsageMeters({
  plan,
  usage,
}: {
  plan: Plan
  usage: SubscriptionUsage
}) {
  const eventsLimit = plan.limits.events
  const storageLimit = plan.limits.storageBytes

  return (
    <div className="grid gap-4 sm:grid-cols-2 sm:gap-6">
      <Meter
        label="Events"
        used={usage.events}
        limit={eventsLimit}
        valueText={`${usage.events} of ${formatLimit(eventsLimit)}`}
        hint="Counts upcoming, active, and completed events."
      />
      <Meter
        label="Storage"
        used={usage.storageBytes}
        limit={storageLimit}
        valueText={`${formatBytes(usage.storageBytes)} of ${formatByteLimit(storageLimit)}`}
        hint="Uploaded originals across your events."
      />
    </div>
  )
}
