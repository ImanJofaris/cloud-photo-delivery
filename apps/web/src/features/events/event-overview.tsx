"use client"

import * as React from "react"

import type { components } from "@workspace/api-client"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import { useEventAnalytics } from "@/features/analytics/api"
import { formatCount } from "@/features/analytics/format"
import { DEFAULT_ANALYTICS_WINDOW } from "@/features/analytics/schema"

import { useEventDashboard } from "./api"
import { ExpiryCard } from "./expiry-card"
import { ExpiryLabel } from "./expiry-label"
import { ExportCard } from "./export-card"
import { formatDate } from "./format"
import { LatestUploads } from "./latest-uploads"
import { QrHeroCard } from "./qr-hero-card"
import { SharePanel } from "./share-panel"
import { EventStatusCard } from "./status-actions"

type Event = components["schemas"]["Event"]

function StatCard({
  label,
  value,
  hint,
}: {
  label: string
  value: React.ReactNode
  hint: string
}) {
  return (
    <Card size="sm" className="gap-1">
      <CardHeader className="gap-1">
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-xl">{value}</CardTitle>
        <p className="text-xs text-muted-foreground">{hint}</p>
      </CardHeader>
    </Card>
  )
}

function DetailsCard({ event, scans }: { event: Event; scans?: number }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Details</CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="grid gap-2 text-sm">
          <div className="flex justify-between gap-4">
            <dt className="text-muted-foreground">Event date</dt>
            <dd>{formatDate(event.eventDate)}</dd>
          </div>
          <div className="flex justify-between gap-4">
            <dt className="text-muted-foreground">Expires</dt>
            <dd className="flex flex-wrap items-center justify-end gap-2">
              <span>{formatDate(event.expiresAt)}</span>
              <ExpiryLabel expiresAt={event.expiresAt} className="text-xs" />
            </dd>
          </div>
          <div className="flex justify-between gap-4">
            <dt className="text-muted-foreground">Created</dt>
            <dd>{formatDate(event.createdAt)}</dd>
          </div>
          {event.location ? (
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">Location</dt>
              <dd className="text-right">{event.location}</dd>
            </div>
          ) : null}
          {typeof scans === "number" ? (
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">QR scans</dt>
              <dd>{formatCount(scans)}</dd>
            </div>
          ) : null}
        </dl>
      </CardContent>
    </Card>
  )
}

export function EventOverview({
  event,
  onShowQr,
  onViewPhotos,
}: {
  event: Event
  onShowQr: () => void
  onViewPhotos: () => void
}) {
  const dashboard = useEventDashboard(event.id)
  const analytics = useEventAnalytics(event.id, DEFAULT_ANALYTICS_WINDOW)
  const totals = analytics.data?.totals
  const scans = totals?.qrScans
  const dashboardPending = dashboard.isPending
  const analyticsPending = analytics.isPending

  return (
    <div className="space-y-4">
      <ExpiryCard
        eventId={event.id}
        status={event.status}
        createdAt={event.createdAt}
        expiresAt={event.expiresAt}
      />

      <div className="grid gap-4 lg:grid-cols-[1.45fr_1fr]">
        <div className="min-w-0 space-y-4">
          <QrHeroCard
            eventId={event.id}
            eventName={event.name}
            scans={scans}
          />

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <StatCard
              label="Photos"
              value={dashboardPending ? "—" : formatCount(dashboard.data?.photoCount)}
              hint="In this gallery"
            />
            <StatCard
              label="Guests"
              value={dashboardPending ? "—" : formatCount(dashboard.data?.guestCount)}
              hint="Unique visitors"
            />
            <StatCard
              label="Downloads"
              value={analyticsPending ? "—" : formatCount(totals?.downloads)}
              hint={`Last ${DEFAULT_ANALYTICS_WINDOW} days`}
            />
            <StatCard
              label="Views"
              value={analyticsPending ? "—" : formatCount(totals?.galleryViews)}
              hint={`Last ${DEFAULT_ANALYTICS_WINDOW} days`}
            />
          </div>

          <LatestUploads
            eventId={event.id}
            total={dashboard.data?.photoCount}
            onViewAll={onViewPhotos}
          />
        </div>

        <div className="min-w-0 space-y-4">
          <SharePanel
            eventId={event.id}
            slug={event.slug}
            onShowQr={onShowQr}
          />
          <DetailsCard event={event} scans={scans} />
          <EventStatusCard eventId={event.id} status={event.status} />
          <ExportCard eventId={event.id} photoCount={dashboard.data?.photoCount} />
        </div>
      </div>
    </div>
  )
}
