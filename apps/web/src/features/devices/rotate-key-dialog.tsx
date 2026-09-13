"use client"

import * as React from "react"
import { toast } from "sonner"

import type { components } from "@workspace/api-client"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@workspace/ui/components/alert-dialog"

import { useRotateDeviceKey } from "./api"
import { deviceErrorMessage } from "./errors"

type Device = components["schemas"]["Device"]
type DeviceWithKey = components["schemas"]["DeviceWithKey"]

export function RotateKeyDialog({
  device,
  onClose,
  onRotated,
}: {
  device: Device | null
  onClose: () => void
  onRotated: (result: DeviceWithKey) => void
}) {
  const rotate = useRotateDeviceKey(device?.id ?? "")

  async function handleRotate() {
    try {
      const result = await rotate.mutateAsync()
      onClose()
      onRotated(result)
    } catch (error) {
      toast.error(deviceErrorMessage(error))
    }
  }

  return (
    <AlertDialog
      open={device !== null}
      onOpenChange={(open) => !open && onClose()}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            Rotate the key for {device?.name}?
          </AlertDialogTitle>
          <AlertDialogDescription>
            The current key stops working immediately. The new key is shown
            once after rotating.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={rotate.isPending}>
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={rotate.isPending}
            onClick={() => void handleRotate()}
          >
            {rotate.isPending ? "Rotating..." : "Rotate key"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
