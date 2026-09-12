"use client"

import { Archive, Trash2 } from "lucide-react"
import { useRouter } from "next/navigation"
import * as React from "react"
import { toast } from "sonner"

import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"

import { useArchiveEvent, useDeleteEvent } from "./api"
import { eventErrorMessage } from "./errors"

export function DangerZone({
  eventId,
  eventName,
  status,
}: {
  eventId: string
  eventName: string
  status: string
}) {
  const router = useRouter()
  const archiveEvent = useArchiveEvent(eventId)
  const deleteEvent = useDeleteEvent(eventId)
  const [confirmName, setConfirmName] = React.useState("")

  async function handleArchive() {
    try {
      await archiveEvent.mutateAsync()
      toast.success("Event archived")
    } catch (error) {
      toast.error(eventErrorMessage(error))
    }
  }

  async function handleDelete() {
    try {
      await deleteEvent.mutateAsync()
      toast.success("Event deleted")
      router.push("/events")
    } catch (error) {
      toast.error(eventErrorMessage(error))
    }
  }

  return (
    <Card className="border-destructive/40">
      <CardHeader>
        <CardTitle>Danger zone</CardTitle>
        <CardDescription>
          Archiving hides the gallery; deleting keeps the photos recoverable
          during the grace period.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-sm font-medium">Archive event</p>
            <p className="text-xs text-muted-foreground">
              The gallery stops accepting new photos.
            </p>
          </div>
          <Button
            variant="outline"
            onClick={() => void handleArchive()}
            disabled={status === "archived" || archiveEvent.isPending}
          >
            <Archive />
            {archiveEvent.isPending ? "Archiving..." : "Archive"}
          </Button>
        </div>

        <div className="space-y-3 border-t pt-6">
          <div>
            <p className="text-sm font-medium">Delete event</p>
            <p className="text-xs text-muted-foreground">
              Type the event name to confirm.
            </p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1 space-y-2">
              <Label htmlFor="confirmName">Event name</Label>
              <Input
                id="confirmName"
                value={confirmName}
                onChange={(changeEvent) =>
                  setConfirmName(changeEvent.target.value)
                }
                placeholder={eventName}
              />
            </div>
            <Button
              variant="destructive"
              disabled={confirmName !== eventName || deleteEvent.isPending}
              onClick={() => void handleDelete()}
            >
              <Trash2 />
              {deleteEvent.isPending ? "Deleting..." : "Delete event"}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
