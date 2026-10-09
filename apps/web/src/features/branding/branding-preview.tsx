"use client"

import { CalendarDays, MapPin } from "lucide-react"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import type { BrandingValues } from "./schema"

const HEX_PATTERN = /^#[0-9a-f]{6}$/
const DEFAULT_PRIMARY = "#64748b"
const DEFAULT_SECONDARY = "#94a3b8"

const PHOTO_PLACEHOLDERS = [
  "from-[#f4cdb2] via-[#e3a07c] to-[#bd6f55]",
  "from-[#f4dfd6] via-[#dbb09a] to-[#b98a74]",
  "from-[#f3d8d8] via-[#e0a8a8] to-[#c88080]",
]

function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  const first = words[0]?.charAt(0) ?? "?"
  const second = words[1]?.charAt(0) ?? ""
  return `${first}${second}`.toUpperCase()
}

export function BrandingPreview({
  values,
  logoUrl,
  profileImageUrl,
}: {
  values: BrandingValues
  logoUrl: string | null
  profileImageUrl: string | null
}) {
  const primary = HEX_PATTERN.test(values.primaryColor)
    ? values.primaryColor
    : DEFAULT_PRIMARY
  const secondary = HEX_PATTERN.test(values.secondaryColor)
    ? values.secondaryColor
    : DEFAULT_SECONDARY
  const businessName = values.businessName.trim() || "Your business"

  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle>Guest preview</CardTitle>
        <CardDescription>Matches the public gallery header.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="overflow-hidden rounded-xl bg-background ring-1 ring-foreground/10">
          <div className="relative h-20 w-full overflow-hidden">
            {profileImageUrl ? (
              // Signed URLs and object URLs change per mount; next/image adds
              // nothing here.
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={profileImageUrl}
                alt=""
                className="size-full object-cover"
                decoding="async"
              />
            ) : (
              <>
                <div
                  className="size-full"
                  style={{
                    background: `linear-gradient(118deg, ${secondary}, ${primary})`,
                  }}
                />
                <div className="absolute inset-0 bg-[radial-gradient(120%_100%_at_18%_8%,rgba(255,255,255,0.35),transparent_58%)]" />
              </>
            )}
          </div>

          <div className="px-4 pb-4">
            <div
              className="-mt-5 mb-2 flex size-9 items-center justify-center overflow-hidden rounded-full text-[11px] font-extrabold text-white ring-3 ring-background"
              style={{ backgroundColor: primary }}
            >
              {logoUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={logoUrl}
                  alt=""
                  className="size-full object-cover"
                  decoding="async"
                />
              ) : (
                initials(businessName)
              )}
            </div>

            <p className="text-xs font-semibold" style={{ color: primary }}>
              {businessName}
            </p>
            <p className="mt-1 text-sm font-bold">Aina &amp; Haziq Wedding</p>
            <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
              <span className="inline-flex items-center gap-1">
                <CalendarDays className="size-3" aria-hidden="true" />
                4 Oct 2026
              </span>
              <span className="inline-flex items-center gap-1">
                <MapPin className="size-3" aria-hidden="true" />
                Dewan Perdana Felda, KL
              </span>
            </div>
            <span className="mt-2 inline-flex h-5 items-center rounded-full bg-muted px-2 text-[10px] font-semibold text-muted-foreground">
              486 photos
            </span>
          </div>
        </div>

        <div className="mt-3 flex gap-2">
          {PHOTO_PLACEHOLDERS.map((gradient) => (
            <div
              key={gradient}
              className={`h-14 flex-1 rounded-lg bg-gradient-to-br ${gradient}`}
            />
          ))}
        </div>

        <p className="mt-2 text-[11px] text-muted-foreground">
          Preview reflects unsaved changes.
        </p>
      </CardContent>
    </Card>
  )
}
