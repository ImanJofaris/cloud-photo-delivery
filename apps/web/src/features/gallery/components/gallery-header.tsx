"use client"

import { CalendarDays, Globe, Mail, MapPin, Phone } from "lucide-react"

import { formatDate } from "@/features/events/format"

import { usePhotoUrlWithFallback } from "../api"
import type { GalleryEvent } from "../types"

export function GalleryHeader({
  slug,
  event,
  viewable,
}: {
  slug: string
  event: GalleryEvent
  viewable: boolean
}) {
  const branding = event.branding ?? null
  const coverPhotoId = event.coverPhotoId ?? null
  const cover = usePhotoUrlWithFallback(
    slug,
    coverPhotoId ?? "",
    ["large", "medium", "thumbnail"],
    Boolean(coverPhotoId) && viewable
  )

  const hasIdentity = Boolean(branding?.logoUrl || branding?.businessName)
  const bannerUrl = cover.data ?? branding?.profileImageUrl ?? null

  return (
    <header>
      <div className="relative h-40 w-full overflow-hidden bg-muted sm:h-56">
        {bannerUrl ? (
          // Signed URLs change per fetch; the Next image optimizer would add
          // latency without caching anything.
          // eslint-disable-next-line @next/next/no-img-element
          <img src={bannerUrl} alt="" className="size-full object-cover" />
        ) : (
          <div className="size-full bg-gradient-to-br from-[var(--gallery-primary)]/25 via-muted to-muted" />
        )}
      </div>

      <div className="mx-auto w-full max-w-3xl space-y-3 px-4 py-5">
        {hasIdentity ? (
          <div className="flex items-center gap-2">
            {branding?.logoUrl ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={branding.logoUrl}
                alt={branding.businessName ?? ""}
                className="size-9 rounded-full border bg-background object-cover"
              />
            ) : null}
            {branding?.businessName ? (
              <span className="text-sm font-medium text-[var(--gallery-primary)]">
                {branding.businessName}
              </span>
            ) : null}
          </div>
        ) : null}

        <div className="space-y-2">
          <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
            {event.name}
          </h1>

          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
            {event.date ? (
              <span className="inline-flex items-center gap-1.5">
                <CalendarDays className="size-4" aria-hidden="true" />
                {formatDate(event.date)}
              </span>
            ) : null}
            {event.location ? (
              <span className="inline-flex items-center gap-1.5">
                <MapPin className="size-4" aria-hidden="true" />
                {event.location}
              </span>
            ) : null}
          </div>

          {event.description ? (
            <p className="max-w-2xl text-sm text-muted-foreground">
              {event.description}
            </p>
          ) : null}

          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
            {branding?.contactEmail ? (
              <a
                className="inline-flex items-center gap-1.5 text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
                href={`mailto:${branding.contactEmail}`}
              >
                <Mail className="size-4" aria-hidden="true" />
                {branding.contactEmail}
              </a>
            ) : null}
            {branding?.contactPhone ? (
              <a
                className="inline-flex items-center gap-1.5 text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
                href={`tel:${branding.contactPhone}`}
              >
                <Phone className="size-4" aria-hidden="true" />
                {branding.contactPhone}
              </a>
            ) : null}
            {branding?.websiteUrl ? (
              <a
                className="inline-flex items-center gap-1.5 text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
                href={branding.websiteUrl}
                target="_blank"
                rel="noreferrer"
              >
                <Globe className="size-4" aria-hidden="true" />
                {branding.websiteUrl.replace(/^https?:\/\//, "")}
              </a>
            ) : null}
          </div>
        </div>
      </div>
    </header>
  )
}
