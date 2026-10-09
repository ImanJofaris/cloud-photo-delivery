"use client"

import { Clock } from "lucide-react"

import type { components } from "@workspace/api-client"
import { Button } from "@workspace/ui/components/button"
import { Card, CardContent } from "@workspace/ui/components/card"
import {
  Progress,
} from "@workspace/ui/components/progress"
import { cn } from "@workspace/ui/lib/utils"

import { ExtendExpiryDialog } from "./extend-expiry-dialog"
import {
  expiryFromNow,
  expiryProgressPercent,
  formatDate,
  formatTimeUntil,
  type ExpiryTone,
} from "./format"

type EventStatus = components["schemas"]["EventStatus"]

export function expiryCardCopy(expiresAt: string | null | undefined): {
  tone: ExpiryTone
  title: string
  description: string
} {
  if (!expiresAt) {
    return {
      tone: "neutral",
      title: "This gallery does not expire",
      description:
        "It stays online until you set an expiry or archive the event.",
    }
  }

  const date = formatDate(expiresAt)
  const expiry = expiryFromNow(expiresAt)
  if (expiry.tone === "expired") {
    return {
      tone: "expired",
      title: `Gallery closed ${date}`,
      description: "Guests see a closed page until you extend the event.",
    }
  }

  const remaining = formatTimeUntil(expiresAt)
  return {
    tone: expiry.tone,
    title: remaining
      ? `Gallery closes ${date} · in ${remaining}`
      : `Gallery closes ${date}`,
    description: "After expiry guests see a closed page until you extend it.",
  }
}

const toneStyles: Record<ExpiryTone, { icon: string; bar: string }> = {
  neutral: {
    icon: "bg-primary/10 text-primary",
    bar: "",
  },
  warning: {
    icon: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
    bar: "[&_[data-slot=progress-indicator]]:bg-amber-500",
  },
  expired: {
    icon: "bg-destructive/10 text-destructive",
    bar: "[&_[data-slot=progress-indicator]]:bg-destructive",
  },
}

export function ExpiryCard({
  eventId,
  status,
  createdAt,
  expiresAt,
}: {
  eventId: string
  status: EventStatus
  createdAt: string
  expiresAt: string | null
}) {
  const copy = expiryCardCopy(expiresAt)
  const percent = expiryProgressPercent(createdAt, expiresAt)
  const styles = toneStyles[copy.tone]

  return (
    <Card className="gap-0 py-0">
      <CardContent className="flex flex-wrap items-center gap-3 px-4 py-3.5 sm:gap-4">
        <div
          className={cn(
            "flex size-10 shrink-0 items-center justify-center rounded-xl",
            styles.icon
          )}
        >
          <Clock className="size-5" aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium">{copy.title}</p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {copy.description}
          </p>
        </div>
        <div className="flex w-full items-center gap-3 sm:w-auto">
          {percent !== null && (
            <Progress
              value={percent}
              aria-label={`${percent}% of the gallery period elapsed`}
              className={cn("hidden w-36 shrink-0 sm:block", styles.bar)}
            />
          )}
          <div className="ml-auto sm:ml-0">
            <ExtendExpiryDialog
              eventId={eventId}
              status={status}
              trigger={
                <Button variant="outline" size="sm">
                  Extend expiry
                </Button>
              }
            />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
