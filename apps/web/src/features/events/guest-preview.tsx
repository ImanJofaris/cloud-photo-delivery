"use client"

import { Download, Lock } from "lucide-react"

import { Badge } from "@workspace/ui/components/badge"
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { cn } from "@workspace/ui/lib/utils"

import { formatDate } from "./format"
import type { EventSettingsValues } from "./schema"

type Visibility = EventSettingsValues["visibility"]

function initialsFor(name: string): string {
  const letters = name
    .split(/\s+/)
    .map((word) => word.replace(/[^\p{L}\p{N}]/gu, ""))
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => word[0])
    .join("")
  return letters.toUpperCase() || "?"
}

export function GuestPreviewCard({
  eventName,
  eventDate,
  location,
  photoCount,
  visibility,
  allowDownload,
  allowOriginalDownload,
  watermarkEnabled,
  primaryColor,
  businessName,
}: {
  eventName: string
  eventDate: string | null | undefined
  location: string | null | undefined
  photoCount: number
  visibility: Visibility
  allowDownload: boolean
  allowOriginalDownload: boolean
  watermarkEnabled: boolean
  primaryColor: string | null
  businessName: string | null | undefined
}) {
  const accent = primaryColor ? { backgroundColor: primaryColor } : undefined
  const displayName = eventName || "Your event"
  const meta = [eventDate ? formatDate(eventDate) : null, location || null]
    .filter(Boolean)
    .join(" · ")

  return (
    <Card>
      <CardHeader>
        <CardTitle>Guest preview</CardTitle>
        <CardAction>
          <Badge variant="secondary">Updates live</Badge>
        </CardAction>
      </CardHeader>
      <CardContent>
        <div className="overflow-hidden rounded-xl bg-card ring-1 ring-foreground/10">
          <div
            className={cn(
              "relative h-20",
              primaryColor ? undefined : "bg-primary"
            )}
            style={accent}
          >
            <span
              aria-hidden="true"
              className="absolute inset-0 bg-gradient-to-br from-white/25 to-transparent"
            />
            {watermarkEnabled ? (
              <span
                aria-hidden="true"
                className="absolute right-3 bottom-2 -rotate-6 text-[10px] font-bold tracking-[0.25em] text-white/80"
              >
                WATERMARK
              </span>
            ) : null}
          </div>
          <div className="px-4 pb-4">
            <div
              className={cn(
                "relative -mt-5 flex size-10 items-center justify-center rounded-full text-xs font-bold text-white ring-3 ring-card",
                primaryColor ? undefined : "bg-primary"
              )}
              style={accent}
            >
              {initialsFor(businessName?.trim() || displayName)}
            </div>
            <p className="mt-2 truncate text-sm font-semibold">{displayName}</p>
            {meta ? (
              <p className="truncate text-xs text-muted-foreground">{meta}</p>
            ) : null}
            <div className="mt-3 flex flex-wrap gap-1.5">
              <Badge variant="secondary">
                {photoCount} {photoCount === 1 ? "photo" : "photos"}
              </Badge>
              <Badge variant="secondary">
                Downloads {allowDownload ? "on" : "off"}
              </Badge>
              {allowDownload && allowOriginalDownload ? (
                <Badge variant="outline">Originals</Badge>
              ) : null}
              {visibility === "password" ? (
                <Badge variant="outline">
                  <Lock data-icon="inline-start" />
                  Password
                </Badge>
              ) : null}
              {visibility === "private" ? (
                <Badge variant="outline">Hidden</Badge>
              ) : null}
            </div>
            {allowDownload ? (
              <span
                aria-hidden="true"
                className={cn(
                  "mt-4 flex h-8 w-full items-center justify-center gap-1.5 rounded-control text-sm font-medium text-primary-foreground",
                  primaryColor ? undefined : "bg-primary"
                )}
                style={accent}
              >
                <Download className="size-4" />
                Download all
              </span>
            ) : null}
          </div>
        </div>
        <p className="mt-3 text-xs text-muted-foreground">
          Reflects your settings before you save.
        </p>
      </CardContent>
    </Card>
  )
}
