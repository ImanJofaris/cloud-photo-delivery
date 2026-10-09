import type { components } from "@workspace/api-client"

import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { busiestDays, formatCount, formatUtcWeekday } from "./format"

type AnalyticsDay = components["schemas"]["AnalyticsDay"]

export function BusiestDays({
  daily,
  loading = false,
}: {
  daily?: AnalyticsDay[]
  loading?: boolean
}) {
  const top = busiestDays(daily ?? [], 3)

  return (
    <Card>
      <CardHeader>
        <CardTitle>Busiest days</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="space-y-3">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-2/3" />
          </div>
        ) : top.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No activity in this window yet.
          </p>
        ) : (
          <ul className="space-y-2.5">
            {top.map((day) => (
              <li key={day.date} className="flex gap-2.5 text-sm leading-snug">
                <span
                  aria-hidden="true"
                  className="mt-1.5 size-1.5 shrink-0 rounded-full bg-primary"
                />
                <p>
                  <span className="font-semibold">
                    {formatUtcWeekday(day.date)}
                  </span>
                  <span aria-hidden="true"> · </span>
                  <span>
                    {`${formatCount(day.galleryViews)} views, ${formatCount(
                      day.downloads
                    )} downloads`}
                  </span>
                </p>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
