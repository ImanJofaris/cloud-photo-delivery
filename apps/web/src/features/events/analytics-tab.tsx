"use client"

import * as React from "react"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { AnalyticsTotals } from "@/features/analytics/analytics-totals"
import { useEventAnalytics } from "@/features/analytics/api"
import { DailyChart } from "@/features/analytics/daily-chart"
import { analyticsErrorMessage } from "@/features/analytics/errors"
import { DEFAULT_ANALYTICS_WINDOW } from "@/features/analytics/schema"
import { WindowSelect } from "@/features/analytics/window-select"

export function EventAnalyticsTab({ eventId }: { eventId: string }) {
  const [days, setDays] = React.useState(DEFAULT_ANALYTICS_WINDOW)
  const query = useEventAnalytics(eventId, days)
  const data = query.data

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          Gallery activity for this event.
        </p>
        <WindowSelect days={days} onDaysChange={setDays} />
      </div>

      {query.isError ? (
        <Card>
          <CardContent className="py-8 text-sm text-destructive">
            {analyticsErrorMessage(query.error)}
          </CardContent>
        </Card>
      ) : (
        <>
          <AnalyticsTotals
            totals={data?.totals}
            photoCount={data?.photoCount}
            loading={query.isPending}
          />

          <Card>
            <CardHeader>
              <CardTitle>Last {days} days</CardTitle>
              <CardDescription>
                Views and interactions per UTC day.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {query.isPending || !data ? (
                <Skeleton className="h-64 w-full" />
              ) : (
                <DailyChart daily={data.daily} days={days} />
              )}
            </CardContent>
          </Card>

          <p className="text-xs text-muted-foreground">
            Counters are recorded per UTC day. Unique visitors is the sum of
            daily uniques, so a returning visitor counts again on a new day.
            Downloads only accrue while original downloads are enabled.
          </p>
        </>
      )}
    </div>
  )
}
