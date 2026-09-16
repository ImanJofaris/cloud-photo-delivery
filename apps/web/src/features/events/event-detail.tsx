"use client"

import { ArrowLeft, TriangleAlert } from "lucide-react"
import Link from "next/link"

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@workspace/ui/components/tabs"

import { PhotosPanel } from "@/features/photos/photos-panel"
import { UploadProvider } from "@/features/photos/upload-provider"

import {
  isNotFound,
  useEvent,
  useEventDashboard,
  useEventSettings,
} from "./api"
import { EventAnalyticsTab } from "./analytics-tab"
import { DangerZone } from "./danger-zone"
import { ExpiryLabel } from "./expiry-label"
import { ExportCard } from "./export-card"
import { ExtendExpiryDialog } from "./extend-expiry-dialog"
import { formatBytes, formatDate, formatDateTime } from "./format"
import { EventSettingsForm } from "./settings-form"
import { GalleryLinkPreview, SharePanel } from "./share-panel"
import type { EventSettingsValues } from "./schema"
import { EventStatusBadge } from "./status-badge"
import { EventStatusCard } from "./status-actions"

function toSettingsDefaults(
  settings: ReturnType<typeof useEventSettings>["data"]
): EventSettingsValues | null {
  if (!settings) return null
  return {
    visibility: settings.visibility,
    password: "",
    allowDownload: settings.allowDownload,
    allowOriginalDownload: settings.allowOriginalDownload,
    watermarkEnabled: settings.watermarkEnabled,
  }
}

export function EventDetail({ eventId }: { eventId: string }) {
  const eventQuery = useEvent(eventId)
  const dashboardQuery = useEventDashboard(eventId)
  const settingsQuery = useEventSettings(eventId)

  if (eventQuery.isPending) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  }

  if (eventQuery.isError) {
    return (
      <Card className="max-w-xl">
        <CardHeader>
          <CardTitle>
            {isNotFound(eventQuery.error)
              ? "Event not found"
              : "Something went wrong"}
          </CardTitle>
          <CardDescription>
            {isNotFound(eventQuery.error)
              ? "This event does not exist or belongs to another account."
              : "Could not load the event. Please try again."}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            variant="outline"
            render={<Link href="/events" />}
            nativeButton={false}
          >
            <ArrowLeft />
            Back to events
          </Button>
        </CardContent>
      </Card>
    )
  }

  const event = eventQuery.data
  const dashboard = dashboardQuery.data
  const settingsDefaults = toSettingsDefaults(settingsQuery.data)

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Button
          variant="ghost"
          size="sm"
          render={<Link href="/events" />}
          nativeButton={false}
        >
          <ArrowLeft />
          Events
        </Button>
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-2xl font-semibold">{event.name}</h1>
          <EventStatusBadge status={event.status} />
        </div>
        <p className="text-sm text-muted-foreground">
          {event.clientName ? `${event.clientName} · ` : ""}
          {formatDate(event.eventDate)} · Created {formatDate(event.createdAt)}
          {" · "}
          <GalleryLinkPreview slug={event.slug} />
        </p>
      </div>

      <UploadProvider eventId={event.id}>
        <Tabs defaultValue="overview" className="gap-6">
          <TabsList variant="line">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="analytics">Analytics</TabsTrigger>
            <TabsTrigger value="photos">Photos</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
            <TabsTrigger value="danger">Danger zone</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-6">
            {event.status === "expired" && (
              <Alert variant="destructive">
                <TriangleAlert />
                <AlertTitle>This event expired</AlertTitle>
                <AlertDescription className="flex flex-wrap items-center gap-3">
                  <span>
                    This event expired on {formatDate(event.expiresAt)}. Its
                    gallery is no longer available. Extend to reactivate.
                  </span>
                  <ExtendExpiryDialog
                    eventId={event.id}
                    status={event.status}
                    trigger={
                      <Button variant="outline" size="sm">
                        Extend
                      </Button>
                    }
                  />
                </AlertDescription>
              </Alert>
            )}

            <EventStatusCard eventId={event.id} status={event.status} />

            <div className="grid gap-4 sm:grid-cols-3">
              <Card>
                <CardHeader className="pb-2">
                  <CardDescription>Photos</CardDescription>
                  <CardTitle className="text-2xl">
                    {dashboard ? dashboard.photoCount : "—"}
                  </CardTitle>
                </CardHeader>
              </Card>
              <Card>
                <CardHeader className="pb-2">
                  <CardDescription>Storage</CardDescription>
                  <CardTitle className="text-2xl">
                    {dashboard ? formatBytes(dashboard.storageBytes) : "—"}
                  </CardTitle>
                </CardHeader>
              </Card>
              <Card>
                <CardHeader className="pb-2">
                  <CardDescription>Guests</CardDescription>
                  <CardTitle className="text-2xl">
                    {dashboard ? dashboard.guestCount : "—"}
                  </CardTitle>
                </CardHeader>
              </Card>
            </div>

            <div className="grid gap-6 lg:grid-cols-2">
              <SharePanel
                eventId={event.id}
                eventName={event.name}
                slug={event.slug}
              />
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
                        <span>{formatDateTime(event.expiresAt)}</span>
                        <ExpiryLabel expiresAt={event.expiresAt} />
                        <ExtendExpiryDialog
                          eventId={event.id}
                          status={event.status}
                          trigger={
                            <Button variant="outline" size="xs">
                              Extend
                            </Button>
                          }
                        />
                      </dd>
                    </div>
                    <div className="flex justify-between gap-4">
                      <dt className="text-muted-foreground">Last updated</dt>
                      <dd>{formatDateTime(event.updatedAt)}</dd>
                    </div>
                    {event.location && (
                      <div className="flex justify-between gap-4">
                        <dt className="text-muted-foreground">Location</dt>
                        <dd>{event.location}</dd>
                      </div>
                    )}
                  </dl>
                </CardContent>
              </Card>
            </div>

            <ExportCard eventId={event.id} photoCount={dashboard?.photoCount} />
          </TabsContent>

          <TabsContent value="analytics">
            <EventAnalyticsTab eventId={event.id} />
          </TabsContent>

          <TabsContent value="photos">
            <PhotosPanel eventId={event.id} />
          </TabsContent>

          <TabsContent value="settings">
            {settingsDefaults ? (
              <EventSettingsForm
                eventId={event.id}
                defaults={settingsDefaults}
              />
            ) : (
              <Skeleton className="h-72 w-full" />
            )}
          </TabsContent>

          <TabsContent value="danger">
            <DangerZone
              eventId={event.id}
              eventName={event.name}
              status={event.status}
            />
          </TabsContent>
        </Tabs>
      </UploadProvider>
    </div>
  )
}
