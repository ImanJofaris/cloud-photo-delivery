"use client"

import * as React from "react"
import { toast } from "sonner"

import type { components } from "@workspace/api-client"

import { Button } from "@workspace/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@workspace/ui/components/dialog"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"

import { useExtendEvent } from "./api"
import { extendEventErrorMessage } from "./errors"
import { formatDateTime } from "./format"
import {
  EXTEND_MAX_DAYS,
  EXTEND_MIN_DAYS,
  EXTEND_PRESET_DAYS,
  parseExtendDays,
} from "./schema"

type EventStatus = components["schemas"]["EventStatus"]

export function ExtendExpiryDialog({
  eventId,
  status,
  trigger,
}: {
  eventId: string
  status: EventStatus
  trigger: React.ReactElement
}) {
  const [open, setOpen] = React.useState(false)
  const [value, setValue] = React.useState(String(EXTEND_PRESET_DAYS[1]))
  const [error, setError] = React.useState<string | null>(null)
  const extendEvent = useExtendEvent(eventId)

  if (status === "archived") return null

  async function handleSubmit(submitEvent: React.FormEvent<HTMLFormElement>) {
    submitEvent.preventDefault()
    const days = parseExtendDays(value)

    if (days === null) {
      setError(
        `Enter a whole number between ${EXTEND_MIN_DAYS} and ${EXTEND_MAX_DAYS}.`
      )
      return
    }

    setError(null)
    try {
      const event = await extendEvent.mutateAsync(days)
      toast.success(`Event extended to ${formatDateTime(event.expiresAt)}`)
      setOpen(false)
    } catch (submitError) {
      setError(extendEventErrorMessage(submitError))
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Extend event</DialogTitle>
          <DialogDescription>
            The new expiry runs from the later of now and the current expiry.
            Expired events become active again.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4" noValidate>
          <div className="space-y-2">
            <Label htmlFor={`extend-days-${eventId}`}>Extend by</Label>
            <div className="flex flex-wrap gap-2">
              {EXTEND_PRESET_DAYS.map((preset) => (
                <Button
                  key={preset}
                  type="button"
                  size="sm"
                  variant={value === String(preset) ? "secondary" : "outline"}
                  aria-pressed={value === String(preset)}
                  onClick={() => {
                    setValue(String(preset))
                    setError(null)
                  }}
                >
                  {preset} days
                </Button>
              ))}
            </div>
            <Input
              id={`extend-days-${eventId}`}
              inputMode="numeric"
              value={value}
              aria-invalid={Boolean(error)}
              onChange={(changeEvent) => {
                setValue(changeEvent.target.value)
                setError(null)
              }}
            />
            <p className="text-xs text-muted-foreground">
              Presets or a custom value from {EXTEND_MIN_DAYS} to{" "}
              {EXTEND_MAX_DAYS} days.
            </p>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={extendEvent.isPending}>
              {extendEvent.isPending ? "Extending..." : "Extend event"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
