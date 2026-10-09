"use client"

import Link from "next/link"

import { Button } from "@workspace/ui/components/button"
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

import { useSubscription } from "@/features/billing/api"
import {
  formatByteLimit,
  formatLimit,
  usagePercent,
} from "@/features/billing/format"

import { formatBytes } from "./format"

function UsageRow({
  label,
  used,
  limit,
  formatUsed,
  formatLimitValue,
}: {
  label: string
  used: number
  limit: number
  formatUsed: (value: number) => string
  formatLimitValue: (value: number) => string
}) {
  const unlimited = limit <= 0
  const percent = usagePercent(used, limit)

  return (
    <div>
      <div className="flex items-center justify-between gap-3 text-sm">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium">
          {formatUsed(used)} of {formatLimitValue(limit)}
        </span>
      </div>
      {!unlimited && (
        <Progress
          value={percent}
          aria-label={`${label} usage`}
          className="mt-1.5"
        />
      )}
    </div>
  )
}

export function PlanUsageCard() {
  const subscription = useSubscription()
  const view = subscription.data

  return (
    <Card>
      <CardHeader>
        <CardTitle>Plan usage</CardTitle>
        <CardAction>
          <Button
            variant="link"
            size="sm"
            render={<Link href="/billing" />}
            nativeButton={false}
          >
            View plans
          </Button>
        </CardAction>
        <CardDescription>
          What this event will count against.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {subscription.isPending && <Skeleton className="h-16 w-full" />}
        {subscription.isError && (
          <p className="text-sm text-muted-foreground">
            Could not load your plan right now.
          </p>
        )}
        {view && (
          <>
            <UsageRow
              label="Events"
              used={view.usage.events}
              limit={view.plan.limits.events}
              formatUsed={(value) => String(value)}
              formatLimitValue={formatLimit}
            />
            <UsageRow
              label="Storage"
              used={view.usage.storageBytes}
              limit={view.plan.limits.storageBytes}
              formatUsed={formatBytes}
              formatLimitValue={formatByteLimit}
            />
          </>
        )}
      </CardContent>
    </Card>
  )
}
