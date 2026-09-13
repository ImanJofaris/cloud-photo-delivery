"use client"

import { Globe, Mail, Phone } from "lucide-react"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import type { BrandingValues } from "./schema"

const DEFAULT_PRIMARY = "#0f172a"
const DEFAULT_SECONDARY = "#64748b"

export function BrandingPreview({
  values,
  logoUrl,
  profileImageUrl,
}: {
  values: BrandingValues
  logoUrl: string | null
  profileImageUrl: string | null
}) {
  const primary = values.primaryColor || DEFAULT_PRIMARY
  const secondary = values.secondaryColor || DEFAULT_SECONDARY
  const businessName = values.businessName || "Your business"

  return (
    <Card className="h-fit overflow-hidden">
      <CardHeader>
        <CardTitle>Gallery preview</CardTitle>
        <CardDescription>
          How guests see your branding on the public gallery.
        </CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        <div
          className="relative h-28 w-full"
          style={{
            background: `linear-gradient(135deg, ${primary}33, ${secondary}1f)`,
          }}
        >
          {profileImageUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={profileImageUrl}
              alt=""
              className="size-full object-cover"
              decoding="async"
            />
          ) : null}
        </div>

        <div className="space-y-4 p-4">
          <div className="flex items-center gap-2">
            {logoUrl ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={logoUrl}
                alt=""
                className="size-9 rounded-full border bg-background object-cover"
                decoding="async"
              />
            ) : (
              <div
                className="flex size-9 items-center justify-center rounded-full text-xs font-semibold text-white"
                style={{ backgroundColor: primary }}
              >
                {businessName.slice(0, 1).toUpperCase()}
              </div>
            )}
            <span
              className="truncate text-sm font-medium"
              style={{ color: primary }}
            >
              {businessName}
            </span>
          </div>

          <div>
            <p className="text-lg font-semibold">Your event gallery</p>
            <p className="text-xs text-muted-foreground">
              Photos appear here for your guests.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            {values.contactEmail ? (
              <span className="inline-flex items-center gap-1">
                <Mail className="size-3" />
                {values.contactEmail}
              </span>
            ) : null}
            {values.contactPhone ? (
              <span className="inline-flex items-center gap-1">
                <Phone className="size-3" />
                {values.contactPhone}
              </span>
            ) : null}
            {values.websiteUrl ? (
              <span className="inline-flex items-center gap-1">
                <Globe className="size-3" />
                {values.websiteUrl.replace(/^https?:\/\//, "")}
              </span>
            ) : null}
          </div>

          <div className="flex gap-2">
            <span
              className="h-6 flex-1 rounded-full"
              style={{ backgroundColor: primary }}
            />
            <span
              className="h-6 flex-1 rounded-full"
              style={{ backgroundColor: secondary }}
            />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
