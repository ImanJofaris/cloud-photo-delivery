"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import * as React from "react"
import { Controller, useForm } from "react-hook-form"

import type { components } from "@workspace/api-client"

import { Alert, AlertDescription } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@workspace/ui/components/dialog"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select"

import { useCreateDevice } from "./api"
import { deviceErrorMessage } from "./errors"
import {
  createDeviceSchema,
  NO_EVENT,
  toCreateDeviceRequest,
  type CreateDeviceValues,
} from "./schema"

type DeviceWithKey = components["schemas"]["DeviceWithKey"]
type EventOption = { id: string; name: string }

export function DeviceFormDialog({
  open,
  onOpenChange,
  events,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  events: EventOption[]
  onCreated: (result: DeviceWithKey) => void
}) {
  const createDevice = useCreateDevice()
  const form = useForm<CreateDeviceValues>({
    resolver: zodResolver(createDeviceSchema),
    defaultValues: { name: "", assignedEventId: NO_EVENT },
  })

  React.useEffect(() => {
    if (open) {
      form.reset({ name: "", assignedEventId: NO_EVENT })
    }
  }, [open, form])

  async function onSubmit(values: CreateDeviceValues) {
    try {
      const created = await createDevice.mutateAsync(
        toCreateDeviceRequest(values)
      )
      onOpenChange(false)
      onCreated(created)
    } catch (error) {
      form.setError("root", { message: deviceErrorMessage(error) })
    }
  }

  const { errors, isSubmitting } = form.formState

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Add device</DialogTitle>
          <DialogDescription>
            Register a photobooth. The API key is shown once after creating.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className="space-y-4"
          noValidate
        >
          <div className="space-y-2">
            <Label htmlFor="device-name">Device name</Label>
            <Input
              id="device-name"
              placeholder="Booth 1 — main hall"
              aria-invalid={Boolean(errors.name)}
              {...form.register("name")}
            />
            {errors.name && (
              <p className="text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="device-event">Assigned event</Label>
            <Controller
              control={form.control}
              name="assignedEventId"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger id="device-event" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NO_EVENT}>
                      No event (unscoped)
                    </SelectItem>
                    {events.map((event) => (
                      <SelectItem key={event.id} value={event.id}>
                        {event.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
            <p className="text-xs text-muted-foreground">
              Only active events can be assigned. Uploads from the device are
              limited to that event.
            </p>
          </div>

          {errors.root && (
            <Alert variant="destructive">
              <AlertDescription>{errors.root.message}</AlertDescription>
            </Alert>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Creating..." : "Create device"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
