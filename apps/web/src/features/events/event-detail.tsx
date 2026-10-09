"use client"

import { ArrowLeft, Share2 } from "lucide-react"
import Link from "next/link"
import * as React from "react"

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

import { isNotFound, useEvent, useEventSettings } from "./api"
import { EventAnalyticsTab } from "./analytics-tab"
import { DangerZone } from "./danger-zone"
import { EventOverview } from "./event-overview"
import { formatDate } from "./format"
import { QrDialog } from "./qr-dialog"
import type { EventSettingsValues } from "./schema"
import { EventSettingsForm } from "./settings-form"
import { OpenGalleryLink } from "./share-panel"
import { EventStatusBadge } from "./status-badge"

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
  const settingsQuery = useEventSettings(eventId)
  const [tab, setTab] = React.useState("overview")
  const [qrOpen, setQrOpen] = React.useState(false)

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
  const settingsDefaults = toSettingsDefaults(settingsQuery.data)
  const meta = [
    event.clientName,
    event.eventDate ? formatDate(event.eventDate) : null,
    event.location,
  ]
    .filter(Boolean)
    .join(" · ")

  return (
    <div className="space-y-4">
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
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-2xl font-semibold">{event.name}</h1>
              <EventStatusBadge status={event.status} />
            </div>
            {meta && (
              <p className="text-sm text-muted-foreground">{meta}</p>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <OpenGalleryLink slug={event.slug} />
            <Button onClick={() => setQrOpen(true)}>
              <Share2 />
              Share
            </Button>
          </div>
        </div>
      </div>

      <UploadProvider eventId={event.id}>
        <Tabs value={tab} onValueChange={setTab} className="gap-6">
          <TabsList variant="line">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="photos">Photos</TabsTrigger>
            <TabsTrigger value="analytics">Analytics</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>

          <TabsContent value="overview">
            <EventOverview
              event={event}
              onShowQr={() => setQrOpen(true)}
              onViewPhotos={() => setTab("photos")}
            />
          </TabsContent>

          <TabsContent value="photos">
            <PhotosPanel eventId={event.id} />
          </TabsContent>

          <TabsContent value="analytics">
            <EventAnalyticsTab eventId={event.id} />
          </TabsContent>

          <TabsContent value="settings" className="space-y-6">
            {settingsDefaults ? (
              <EventSettingsForm
                eventId={event.id}
                defaults={settingsDefaults}
              />
            ) : (
              <Skeleton className="h-72 w-full" />
            )}
            <DangerZone
              eventId={event.id}
              eventName={event.name}
              status={event.status}
            />
          </TabsContent>
        </Tabs>
      </UploadProvider>

      <QrDialog
        eventId={event.id}
        eventName={event.name}
        open={qrOpen}
        onOpenChange={setQrOpen}
      />
    </div>
  )
}
