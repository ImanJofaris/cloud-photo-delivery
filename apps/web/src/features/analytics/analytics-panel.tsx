"use client"

import * as React from "react"

import { Card, CardContent } from "@workspace/ui/components/card"

import { AllTimeCard } from "./all-time-card"
import { AnalyticsTotals } from "./analytics-totals"
import { useAccountAnalytics } from "./api"
import { BusiestDays } from "./busiest-days"
import { analyticsErrorMessage } from "./errors"
import { formatUtcWindowRange } from "./format"
import { DEFAULT_ANALYTICS_WINDOW } from "./schema"
import { ViewsDownloadsCard } from "./views-downloads-card"
import { WindowTabs } from "./window-tabs"

export function AnalyticsPanel() {
  const [days, setDays] = React.useState(DEFAULT_ANALYTICS_WINDOW)
  const query = useAccountAnalytics(days)
  const data = query.data

  const captions = {
    galleryViews: formatUtcWindowRange(days),
    uniqueVisitors: "sum of daily uniques",
    downloads: "original downloads",
    qrScans: "gallery opens via QR",
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">Analytics</h1>
          <p className="text-sm text-muted-foreground">
            Gallery activity across all of your events.
          </p>
        </div>
        <WindowTabs days={days} onDaysChange={setDays} />
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
            loading={query.isPending}
            captions={captions}
          />

          <ViewsDownloadsCard
            daily={data?.daily}
            days={days}
            loading={query.isPending}
          />

          <div className="grid gap-4 lg:grid-cols-[1.5fr_1fr]">
            <BusiestDays daily={data?.daily} loading={query.isPending} />
            <AllTimeCard
              eventCount={data?.eventCount}
              photoCount={data?.photoCount}
            />
          </div>

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
