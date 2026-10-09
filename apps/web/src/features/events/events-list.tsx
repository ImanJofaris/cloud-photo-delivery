"use client"

import { CalendarDays, ChevronRight, Plus, Search } from "lucide-react"
import Link from "next/link"
import * as React from "react"

import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
} from "@workspace/ui/components/card"
import { Input } from "@workspace/ui/components/input"
import { Skeleton } from "@workspace/ui/components/skeleton"
import { cn } from "@workspace/ui/lib/utils"

import { useEvents } from "./api"
import { EventCoverThumbnail } from "./event-cover"
import { expiryToneClass } from "./expiry-label"
import { expiryFromNow, formatDate } from "./format"
import { EventStatusBadge } from "./status-badge"

export const STATUS_OPTIONS = [
  "all",
  "upcoming",
  "active",
  "completed",
  "expired",
  "archived",
] as const

type StatusFilter = (typeof STATUS_OPTIONS)[number]

const STATUS_LABELS: Record<StatusFilter, string> = {
  all: "All",
  upcoming: "Upcoming",
  active: "Active",
  completed: "Completed",
  expired: "Expired",
  archived: "Archived",
}

export function EventsList() {
  const [search, setSearch] = React.useState("")
  const deferredSearch = React.useDeferredValue(search)
  const [status, setStatus] = React.useState<StatusFilter>("all")

  const query = useEvents({ q: deferredSearch.trim(), status })
  const events = query.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">Events</h1>
          <p className="text-sm text-muted-foreground">
            Manage your event galleries.
          </p>
        </div>
        <Button render={<Link href="/events/new" />} nativeButton={false}>
          <Plus />
          New event
        </Button>
      </div>

      <div className="flex items-center gap-3">
        <div className="relative w-full sm:max-w-xs">
          <Search
            className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            placeholder="Search events"
            aria-label="Search events"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            className="pl-8"
          />
        </div>
      </div>

      <div
        role="group"
        aria-label="Filter events by status"
        className="flex flex-wrap gap-2"
      >
        {STATUS_OPTIONS.map((option) => (
          <button
            key={option}
            type="button"
            aria-pressed={status === option}
            onClick={() => setStatus(option)}
            className={cn(
              "inline-flex h-8 items-center rounded-full px-3.5 text-sm font-medium ring-1 ring-foreground/10 transition-colors focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
              status === option
                ? "bg-primary text-primary-foreground ring-transparent"
                : "bg-card text-muted-foreground hover:text-foreground"
            )}
          >
            {STATUS_LABELS[option]}
          </button>
        ))}
      </div>

      {query.isPending && (
        <div className="space-y-3">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      )}

      {query.isError && (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-10 text-center">
            <p className="text-sm text-destructive">
              Could not load events. Please try again.
            </p>
            <Button variant="outline" onClick={() => void query.refetch()}>
              Try again
            </Button>
          </CardContent>
        </Card>
      )}

      {!query.isPending && !query.isError && events.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-12 text-center">
            <CalendarDays className="size-8 text-muted-foreground" />
            <div>
              <p className="font-medium">No events found</p>
              <p className="text-sm text-muted-foreground">
                {deferredSearch
                  ? "Try a different search term or status filter."
                  : "Create your first event to get started."}
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {events.length > 0 && (
        <div className="space-y-4">
          <Card className="gap-0 overflow-hidden py-0">
            <ul className="divide-y divide-border">
              {events.map((event) => {
                const expiry = expiryFromNow(event.expiresAt)

                return (
                  <li key={event.id}>
                    <Link
                      href={`/events/${event.id}`}
                      className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-muted/40 focus-visible:bg-muted/40 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset focus-visible:outline-none"
                    >
                      <EventCoverThumbnail coverPhotoId={event.coverPhotoId} />
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="truncate font-medium">
                            {event.name}
                          </span>
                          <EventStatusBadge status={event.status} />
                        </div>
                        <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-muted-foreground">
                          {event.clientName ? (
                            <span>{event.clientName}</span>
                          ) : null}
                          <span>{formatDate(event.eventDate)}</span>
                          {event.location ? (
                            <span className="truncate">{event.location}</span>
                          ) : null}
                        </div>
                      </div>
                      <div className="hidden shrink-0 items-center gap-4 text-xs text-muted-foreground sm:flex">
                        <span>
                          {event.photoCount}{" "}
                          {event.photoCount === 1 ? "photo" : "photos"}
                        </span>
                        {expiry.tone !== "neutral" && (
                          <span className={expiryToneClass(expiry.tone)}>
                            {expiry.label}
                          </span>
                        )}
                      </div>
                      <ChevronRight
                        className="size-4 shrink-0 text-muted-foreground/60"
                        aria-hidden="true"
                      />
                    </Link>
                  </li>
                )
              })}
            </ul>
          </Card>

          {query.hasNextPage && (
            <div className="flex justify-center">
              <Button
                variant="outline"
                onClick={() => void query.fetchNextPage()}
                disabled={query.isFetchingNextPage}
              >
                {query.isFetchingNextPage ? "Loading..." : "Load more"}
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
