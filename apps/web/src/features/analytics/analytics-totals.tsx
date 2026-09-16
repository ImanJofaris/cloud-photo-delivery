import type { components } from "@workspace/api-client"

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import { formatCount } from "./format"

type AnalyticsCounters = components["schemas"]["AnalyticsCounters"]

export const ANALYTICS_COUNTERS: {
  key: keyof AnalyticsCounters
  label: string
}[] = [
  { key: "galleryViews", label: "Gallery views" },
  { key: "uniqueVisitors", label: "Unique visitors" },
  { key: "downloads", label: "Downloads" },
  { key: "qrScans", label: "QR scans" },
]

export function AnalyticsTotals({
  totals,
  eventCount,
  photoCount,
  loading = false,
}: {
  totals?: AnalyticsCounters
  eventCount?: number
  photoCount?: number
  loading?: boolean
}) {
  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {ANALYTICS_COUNTERS.map((counter) => (
          <Card key={counter.key}>
            <CardHeader className="pb-2">
              <CardDescription>{counter.label}</CardDescription>
              <CardTitle className="text-2xl">
                {loading || !totals ? "—" : formatCount(totals[counter.key])}
              </CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      {(eventCount !== undefined || photoCount !== undefined) && (
        <div className="grid gap-4 sm:grid-cols-2">
          {eventCount !== undefined && (
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>Events (all time)</CardDescription>
                <CardTitle className="text-2xl">
                  {loading ? "—" : formatCount(eventCount)}
                </CardTitle>
              </CardHeader>
            </Card>
          )}
          {photoCount !== undefined && (
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>Photos (all time)</CardDescription>
                <CardTitle className="text-2xl">
                  {loading ? "—" : formatCount(photoCount)}
                </CardTitle>
              </CardHeader>
            </Card>
          )}
        </div>
      )}
    </div>
  )
}
