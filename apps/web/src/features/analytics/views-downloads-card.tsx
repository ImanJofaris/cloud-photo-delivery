"use client"

import * as React from "react"
import { Area, AreaChart, CartesianGrid, XAxis } from "recharts"

import type { components } from "@workspace/api-client"

import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@workspace/ui/components/chart"
import { Skeleton } from "@workspace/ui/components/skeleton"

import {
  fillDailySeries,
  formatCount,
  formatUtcDay,
  formatUtcDayLong,
  seriesTotal,
} from "./format"

type AnalyticsDay = components["schemas"]["AnalyticsDay"]

export const VIEWS_DOWNLOADS_SERIES = [
  { key: "galleryViews", label: "Views", swatchClassName: "bg-chart-1" },
  { key: "downloads", label: "Downloads", swatchClassName: "bg-chart-2" },
] as const

const chartConfig = {
  galleryViews: { label: "Views", color: "var(--chart-1)" },
  downloads: { label: "Downloads", color: "var(--chart-2)" },
} satisfies ChartConfig

export function ViewsDownloadsCard({
  daily,
  days,
  loading = false,
}: {
  daily?: AnalyticsDay[]
  days: number
  loading?: boolean
}) {
  const points = React.useMemo(
    () => fillDailySeries(daily ?? [], days),
    [daily, days]
  )
  const hasActivity = points.some(
    (point) => point.galleryViews > 0 || point.downloads > 0
  )
  const viewTotal = seriesTotal(points, "galleryViews")
  const downloadTotal = seriesTotal(points, "downloads")

  return (
    <Card>
      <CardHeader>
        <CardTitle>Views and downloads</CardTitle>
        <CardAction>
          <ul className="flex items-center gap-4 text-xs text-muted-foreground">
            {VIEWS_DOWNLOADS_SERIES.map((series) => (
              <li key={series.key} className="flex items-center gap-1.5">
                <span
                  aria-hidden="true"
                  className={`size-2.5 rounded-[3px] ${series.swatchClassName}`}
                />
                {series.label}
              </li>
            ))}
          </ul>
        </CardAction>
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-72 w-full" />
        ) : hasActivity ? (
          <>
            <ChartContainer
              config={chartConfig}
              className="aspect-auto h-72 w-full"
            >
              <AreaChart accessibilityLayer data={points}>
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey="date"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  minTickGap={24}
                  tickFormatter={(value: string) => formatUtcDay(value)}
                />
                <ChartTooltip
                  content={
                    <ChartTooltipContent
                      labelFormatter={(value) =>
                        formatUtcDayLong(String(value))
                      }
                    />
                  }
                />
                <Area
                  dataKey="galleryViews"
                  type="monotone"
                  stroke="var(--color-galleryViews)"
                  fill="var(--color-galleryViews)"
                  fillOpacity={0.2}
                  strokeWidth={2}
                  dot={false}
                />
                <Area
                  dataKey="downloads"
                  type="monotone"
                  stroke="var(--color-downloads)"
                  fill="var(--color-downloads)"
                  fillOpacity={0.25}
                  strokeWidth={2}
                  dot={false}
                />
              </AreaChart>
            </ChartContainer>

            <p className="sr-only">
              {`Views: ${formatCount(viewTotal)} total, downloads: ${formatCount(
                downloadTotal
              )} total over the last ${days} UTC days.`}
            </p>
          </>
        ) : (
          <div className="flex h-72 flex-col items-center justify-center gap-1 rounded-lg border border-dashed text-center">
            <p className="text-sm font-medium">No activity yet</p>
            <p className="text-xs text-muted-foreground">
              Views and downloads appear here once guests visit.
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
