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

import { AnalyticsTotals } from "./analytics-totals"
import { useAccountAnalytics } from "./api"
import { DailyChart } from "./daily-chart"
import { analyticsErrorMessage } from "./errors"
import { DEFAULT_ANALYTICS_WINDOW } from "./schema"
import { WindowSelect } from "./window-select"

export function AnalyticsPanel() {
  const [days, setDays] = React.useState(DEFAULT_ANALYTICS_WINDOW)
  const query = useAccountAnalytics(days)
  const data = query.data

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">Analytics</h1>
          <p className="text-sm text-muted-foreground">
            Gallery activity across all of your events.
          </p>
        </div>
        <WindowSelect days={days} onDaysChange={setDays} />
      </div>

      {query.isError && (
        <Card>
          <CardContent className="py-8 text-sm text-destructive">
            {analyticsErrorMessage(query.error)}
          </CardContent>
        </Card>
      )}

      {!query.isError && (
        <>
          <AnalyticsTotals
            totals={data?.totals}
            eventCount={data?.eventCount}
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
