"use client"

import type { components } from "@workspace/api-client"

import { Progress } from "@workspace/ui/components/progress"

import { formatBytes } from "@/features/events/format"

import { formatByteLimit, formatLimit, usagePercent } from "./format"

type Plan = components["schemas"]["Plan"]
type SubscriptionUsage = components["schemas"]["SubscriptionUsage"]

function Meter({
  label,
  used,
  limit,
  valueText,
}: {
  label: string
  used: number
  limit: number
  valueText: string
}) {
  const unlimited = limit === 0
  const over = !unlimited && used > limit
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-sm">
        <span className="text-muted-foreground">{label}</span>
        <span className={over ? "font-medium text-destructive" : undefined}>
          {valueText}
        </span>
      </div>
      {!unlimited && (
        <Progress
          value={usagePercent(used, limit)}
          className={
            over ? "[&_[data-slot=progress-indicator]]:bg-destructive" : undefined
          }
        />
      )}
      {over && (
        <p className="text-xs text-destructive">
          Over the plan limit. Upgrade to unblock new actions.
        </p>
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
  const activeLimit = plan.limits.activeEvents
  const storageLimit = plan.limits.storageBytes

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <Meter
        label="Active events"
        used={usage.activeEvents}
        limit={activeLimit}
        valueText={`${usage.activeEvents} of ${formatLimit(activeLimit)}`}
      />
      <Meter
        label="Storage"
        used={usage.storageBytes}
        limit={storageLimit}
        valueText={`${formatBytes(usage.storageBytes)} of ${formatByteLimit(storageLimit)}`}
      />
    </div>
  )
}
