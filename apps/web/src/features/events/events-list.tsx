"use client"

import { CalendarDays, Plus } from "lucide-react"
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
import { Input } from "@workspace/ui/components/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { useEvents } from "./api"
import { formatDate } from "./format"
import { EventStatusBadge } from "./status-badge"

const STATUS_OPTIONS = [
  "all",
  "upcoming",
  "active",
  "completed",
  "archived",
] as const

type StatusFilter = (typeof STATUS_OPTIONS)[number]

export function EventsList() {
  const [search, setSearch] = React.useState("")
  const deferredSearch = React.useDeferredValue(search)
  const [status, setStatus] = React.useState<StatusFilter>("all")

  const query = useEvents({ q: deferredSearch.trim(), status })
  const events = query.data?.pages.flatMap((page) => page.items) ?? []

  return (
    <div className="space-y-6">
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

      <div className="flex flex-col gap-3 sm:flex-row">
        <Input
          placeholder="Search events"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          className="sm:max-w-xs"
        />
        <Select
          value={status}
          onValueChange={(value) => setStatus(value as StatusFilter)}
        >
          <SelectTrigger className="sm:w-44">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            {STATUS_OPTIONS.map((option) => (
              <SelectItem key={option} value={option}>
                {option === "all"
                  ? "All statuses"
                  : option.charAt(0).toUpperCase() + option.slice(1)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {query.isPending && (
        <div className="space-y-3">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
      )}

      {query.isError && (
        <Card>
          <CardContent className="py-8 text-sm text-destructive">
            Could not load events. Please try again.
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
                  ? "Try a different search term."
                  : "Create your first event to get started."}
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {events.length > 0 && (
        <div className="space-y-3">
          {events.map((event) => (
            <Link
              key={event.id}
              href={`/events/${event.id}`}
              className="block rounded-lg focus-visible:outline-none"
            >
              <Card className="transition-colors hover:border-foreground/20">
                <CardHeader className="gap-1">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <CardTitle className="text-base">{event.name}</CardTitle>
                    <EventStatusBadge status={event.status} />
                  </div>
                  <CardDescription>
                    {event.clientName ? `${event.clientName} · ` : ""}
                    {formatDate(event.eventDate)} · {event.photoCount} photos
                  </CardDescription>
                </CardHeader>
              </Card>
            </Link>
          ))}

          {query.hasNextPage && (
            <div className="flex justify-center pt-2">
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
