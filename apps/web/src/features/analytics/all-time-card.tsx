"use client"

import { Card, CardHeader, CardTitle } from "@workspace/ui/components/card"

import { useSubscription } from "@/features/billing/api"
import { formatBytes } from "@/features/events/format"

import { formatCount } from "./format"

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between border-t px-4 py-2.5 first:border-t-0">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-medium tabular-nums">{value}</dd>
    </div>
  )
}

export function AllTimeCard({
  eventCount,
  photoCount,
}: {
  eventCount?: number
  photoCount?: number
}) {
  const subscription = useSubscription()
  const storageBytes = subscription.data?.usage.storageBytes

  return (
    <Card>
      <CardHeader>
        <CardTitle>All time</CardTitle>
      </CardHeader>
      <dl className="text-sm">
        <Row label="Events" value={formatCount(eventCount)} />
        <Row label="Photos" value={formatCount(photoCount)} />
        <Row
          label="Storage"
          value={storageBytes === undefined ? "—" : formatBytes(storageBytes)}
        />
      </dl>
    </Card>
  )
}
