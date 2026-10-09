"use client"

import Link from "next/link"

import type { components } from "@workspace/api-client"

import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Progress } from "@workspace/ui/components/progress"
import { Skeleton } from "@workspace/ui/components/skeleton"

import {
  formatByteLimit,
  formatLimit,
  usagePercent,
} from "@/features/billing/format"
import { formatBytes } from "@/features/events/format"

import { planSummary } from "./format"

type SubscriptionView = components["schemas"]["SubscriptionView"]

function UsageBar({
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
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="text-muted-foreground">{label}</span>
        <span
          className={over ? "font-medium text-destructive" : "font-medium"}
        >
          {valueText}
        </span>
      </div>
      {!unlimited && (
        <Progress
          value={usagePercent(used, limit)}
          aria-label={label}
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

export function PlanSummaryCard({
  view,
  loading,
  error,
}: {
  view?: SubscriptionView
  loading: boolean
  error: boolean
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Plan</CardTitle>
        {view && <CardDescription>{planSummary(view)}</CardDescription>}
        <CardAction>
          <Link
            href="/billing"
            className="rounded-sm text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            Manage
          </Link>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-3">
        {loading && <Skeleton className="h-20 w-full" />}

        {error && (
          <p className="text-sm text-destructive">
            Could not load your plan right now.
          </p>
        )}

        {view && (
          <>
            <UsageBar
              label="Storage"
              used={view.usage.storageBytes}
              limit={view.plan.limits.storageBytes}
              valueText={`${formatBytes(view.usage.storageBytes)} of ${formatByteLimit(
                view.plan.limits.storageBytes
              )}`}
            />
            <UsageBar
              label="Events"
              used={view.usage.events}
              limit={view.plan.limits.events}
              valueText={`${view.usage.events} of ${formatLimit(
                view.plan.limits.events
              )}`}
            />
          </>
        )}
      </CardContent>
    </Card>
  )
}
