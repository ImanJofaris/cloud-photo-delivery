"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import * as React from "react"
import { useForm } from "react-hook-form"
import { toast } from "sonner"

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

import { useRenameDevice } from "./api"
import { deviceErrorMessage } from "./errors"
import { deviceNameSchema, type DeviceNameValues } from "./schema"

type Device = components["schemas"]["Device"]

export function RenameDeviceDialog({
  device,
  onClose,
}: {
  device: Device | null
  onClose: () => void
}) {
  const renameDevice = useRenameDevice(device?.id ?? "")
  const form = useForm<DeviceNameValues>({
    resolver: zodResolver(deviceNameSchema),
    defaultValues: { name: "" },
  })

  React.useEffect(() => {
    if (device) {
      form.reset({ name: device.name })
    }
  }, [device, form])

  async function onSubmit(values: DeviceNameValues) {
    try {
      await renameDevice.mutateAsync({ name: values.name })
      toast.success("Device renamed")
      onClose()
    } catch (error) {
      form.setError("root", { message: deviceErrorMessage(error) })
    }
  }

  const { errors, isSubmitting } = form.formState

  return (
    <Dialog open={device !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Rename device</DialogTitle>
          <DialogDescription>
            Renaming only changes the label. To change the assigned event,
            create a new device and revoke this one.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className="space-y-4"
          noValidate
        >
          <div className="space-y-2">
            <Label htmlFor="rename-device">Device name</Label>
            <Input
              id="rename-device"
              aria-invalid={Boolean(errors.name)}
              {...form.register("name")}
            />
            {errors.name && (
              <p className="text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>

          {errors.root && (
            <Alert variant="destructive">
              <AlertDescription>{errors.root.message}</AlertDescription>
            </Alert>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Saving..." : "Save name"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
