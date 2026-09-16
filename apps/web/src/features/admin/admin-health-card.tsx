import type { components } from "@workspace/api-client"

import { Badge } from "@workspace/ui/components/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { formatCount } from "@/features/analytics/format"
import { formatDateTime } from "@/features/events/format"

import { formatPendingAge } from "./format"

type AdminHealth = components["schemas"]["AdminHealth"]

export function AdminHealthCard({
  health,
  loading = false,
}: {
  health?: AdminHealth
  loading?: boolean
}) {
  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="space-y-1">
            <CardTitle>Queue health</CardTitle>
            <CardDescription>
              Background jobs across the platform.
            </CardDescription>
          </div>
          <Badge
            variant={health?.status === "degraded" ? "destructive" : "secondary"}
          >
            {loading || !health ? "—" : health.status}
          </Badge>
        </div>
      </CardHeader>
      <CardContent>
        {loading || !health ? (
          <Skeleton className="h-24 w-full" />
        ) : (
          <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div>
              <dt className="text-sm text-muted-foreground">Queue depth</dt>
              <dd className="text-lg font-medium">
                {formatCount(health.queueDepth)}
              </dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">Pending</dt>
              <dd className="text-lg font-medium">
                {formatCount(health.queue.pending)}
              </dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">Running</dt>
              <dd className="text-lg font-medium">
                {formatCount(health.queue.running)}
              </dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">Failed</dt>
              <dd className="text-lg font-medium">
                {formatCount(health.queue.failed)}
              </dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">Oldest pending</dt>
              <dd className="text-lg font-medium">
                {formatPendingAge(health.queue.oldestPendingAt)}
              </dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">Checked</dt>
              <dd className="text-lg font-medium">
                {formatDateTime(health.checkedAt)}
              </dd>
            </div>
          </dl>
        )}
      </CardContent>
    </Card>
  )
}
