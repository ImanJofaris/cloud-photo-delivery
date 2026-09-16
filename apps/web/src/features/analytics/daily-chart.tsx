"use client"

import * as React from "react"
import { Area, AreaChart, CartesianGrid, XAxis } from "recharts"

import type { components } from "@workspace/api-client"

import { Button } from "@workspace/ui/components/button"
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@workspace/ui/components/chart"

import {
  fillDailySeries,
  formatCount,
  formatUtcDay,
  formatUtcDayLong,
  seriesTotal,
  type AnalyticsMetric,
} from "./format"

type AnalyticsDay = components["schemas"]["AnalyticsDay"]

export const ANALYTICS_METRICS: {
  key: AnalyticsMetric
  label: string
}[] = [
  { key: "galleryViews", label: "Views" },
  { key: "uniqueVisitors", label: "Visitors" },
  { key: "downloads", label: "Downloads" },
  { key: "qrScans", label: "QR scans" },
]

const chartConfig = {
  galleryViews: { label: "Views", color: "var(--chart-1)" },
  uniqueVisitors: { label: "Visitors", color: "var(--chart-2)" },
  downloads: { label: "Downloads", color: "var(--chart-3)" },
  qrScans: { label: "QR scans", color: "var(--chart-4)" },
} satisfies ChartConfig

export function DailyChart({
  daily,
  days,
}: {
  daily: AnalyticsDay[]
  days: number
}) {
  const [metric, setMetric] = React.useState<AnalyticsMetric>("galleryViews")
  const points = React.useMemo(
    () => fillDailySeries(daily, days),
    [daily, days]
  )

  const hasActivity = points.some((point) =>
    ANALYTICS_METRICS.some((entry) => point[entry.key] > 0)
  )

  if (!hasActivity) {
    return (
      <div className="flex h-64 flex-col items-center justify-center gap-1 rounded-lg border border-dashed text-center">
        <p className="text-sm font-medium">No activity yet</p>
        <p className="text-xs text-muted-foreground">
          Views, downloads, and QR scans appear here once guests visit.
        </p>
      </div>
    )
  }

  const active = ANALYTICS_METRICS.find((entry) => entry.key === metric)

  return (
    <div className="space-y-4">
      <div
        role="group"
        aria-label="Chart metric"
        className="flex flex-wrap gap-1"
      >
        {ANALYTICS_METRICS.map((entry) => (
          <Button
            key={entry.key}
            type="button"
            size="sm"
            variant={metric === entry.key ? "secondary" : "ghost"}
            aria-pressed={metric === entry.key}
            onClick={() => setMetric(entry.key)}
          >
            {entry.label}
          </Button>
        ))}
      </div>

      <ChartContainer config={chartConfig} className="aspect-auto h-64 w-full">
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
                labelFormatter={(value) => formatUtcDayLong(String(value))}
              />
            }
          />
          <Area
            dataKey={metric}
            type="monotone"
            stroke={`var(--color-${metric})`}
            fill={`var(--color-${metric})`}
            fillOpacity={0.25}
            strokeWidth={2}
            dot={false}
          />
        </AreaChart>
      </ChartContainer>

      <p className="sr-only">
        {`${active?.label ?? "Activity"}: ${formatCount(
          seriesTotal(points, metric)
        )} total over the last ${days} UTC days.`}
      </p>
    </div>
  )
}
