"use client"

import { Archive, Trash2 } from "lucide-react"
import { useRouter } from "next/navigation"
import * as React from "react"
import { toast } from "sonner"

import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
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
    <Card>
      <CardHeader>
        <CardTitle className="text-destructive">Danger zone</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-sm font-medium">Archive event</p>
            <p className="text-xs text-muted-foreground">
              Hides the gallery and pauses uploads. Photos stay safe.
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void handleArchive()}
            disabled={status === "archived" || archiveEvent.isPending}
          >
            <Archive />
            {archiveEvent.isPending ? "Archiving..." : "Archive"}
          </Button>
        </div>

        <div className="space-y-3 border-t pt-4">
          <div>
            <p className="text-sm font-medium">Delete event</p>
            <p className="text-xs text-muted-foreground">
              Removes the event, its photos, and every shared link after the
              30-day grace period. This cannot be undone.
            </p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1 space-y-2">
              <Label htmlFor="confirmName" className="sr-only">
                Event name
              </Label>
              <Input
                id="confirmName"
                value={confirmName}
                onChange={(changeEvent) =>
                  setConfirmName(changeEvent.target.value)
                }
                placeholder={`Type ${eventName} to confirm`}
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
