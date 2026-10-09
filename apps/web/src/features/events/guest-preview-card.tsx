"use client"

import Link from "next/link"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import { useBranding } from "@/features/branding/api"
import { useSubscription } from "@/features/billing/api"

import { formatDate } from "./format"

export type GuestPreviewValues = {
  name: string
  eventDate?: string
  location?: string
  visibility: "public" | "password" | "private"
  allowDownload: boolean
  allowOriginalDownload: boolean
  watermarkEnabled: boolean
}

const VISIBILITY_LABELS: Record<GuestPreviewValues["visibility"], string> = {
  public: "Anyone with the link",
  password: "Password protected",
  private: "Private",
}

export function GuestPreviewCard({ values }: { values: GuestPreviewValues }) {
  const subscription = useSubscription()
  const branding = useBranding()

  const brandingAllowed = Boolean(subscription.data?.plan.limits.branding)
  const primaryColor = brandingAllowed
    ? branding.data?.primaryColor ?? null
    : null
  const logoUrl = brandingAllowed ? branding.data?.logoUrl ?? null : null
  const businessName = brandingAllowed
    ? branding.data?.businessName ?? "Your business"
    : "Your business"

  const chips = [
    VISIBILITY_LABELS[values.visibility],
    values.allowDownload ? "Downloads on" : "Downloads off",
    ...(values.watermarkEnabled ? ["Watermarked"] : []),
  ]

  return (
    <Card className="overflow-hidden">
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle>Guest preview</CardTitle>
          <span className="text-xs text-muted-foreground">
            Live from the form
          </span>
        </div>
        <CardDescription>
          The header guests see after scanning the QR or opening the link.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="overflow-hidden rounded-xl ring-1 ring-border">
          <div
            className="h-20 w-full"
            style={{
              background: primaryColor
                ? `linear-gradient(135deg, ${primaryColor}33, ${primaryColor}99)`
                : "linear-gradient(135deg, color-mix(in oklch, var(--primary) 12%, transparent), color-mix(in oklch, var(--primary) 45%, transparent))",
            }}
          />
          <div className="space-y-2 p-3 pt-0">
            {logoUrl ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={logoUrl}
                alt=""
                decoding="async"
                className="-mt-5 size-9 rounded-full object-cover ring-3 ring-card"
              />
            ) : (
              <div
                className="-mt-5 flex size-9 items-center justify-center rounded-full text-xs font-semibold text-white ring-3 ring-card"
                style={{ backgroundColor: primaryColor ?? "var(--primary)" }}
              >
                {(businessName || "Y").slice(0, 1).toUpperCase()}
              </div>
            )}
            <div>
              <p className="font-semibold">
                {values.name.trim() || "Your event name"}
              </p>
              <p className="text-xs text-muted-foreground">
                {values.eventDate ? formatDate(values.eventDate) : "Date to be set"}
                {values.location?.trim() ? ` · ${values.location.trim()}` : ""}
              </p>
              <p className="mt-0.5 text-xs text-muted-foreground">
                by {businessName}
              </p>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {chips.map((chip) => (
                <span
                  key={chip}
                  className="inline-flex h-5 items-center rounded-full bg-muted px-2.5 text-xs font-medium text-muted-foreground"
                >
                  {chip}
                </span>
              ))}
            </div>
          </div>
        </div>
        {subscription.data && !brandingAllowed && (
          <p className="mt-3 text-xs text-muted-foreground">
            Custom branding and colors are available on paid plans.{" "}
            <Link href="/billing" className="font-medium underline">
              View plans
            </Link>
          </p>
        )}
      </CardContent>
    </Card>
  )
}
