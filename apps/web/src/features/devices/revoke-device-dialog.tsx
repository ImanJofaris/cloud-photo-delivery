"use client"

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

import { useRevokeDevice } from "./api"
import { deviceErrorMessage } from "./errors"

type Device = components["schemas"]["Device"]

export function RevokeDeviceDialog({
  device,
  onClose,
}: {
  device: Device | null
  onClose: () => void
}) {
  const revoke = useRevokeDevice(device?.id ?? "")

  async function handleRevoke() {
    try {
      await revoke.mutateAsync()
      toast.success("Device revoked")
      onClose()
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
          <AlertDialogTitle>Revoke {device?.name}?</AlertDialogTitle>
          <AlertDialogDescription>
            The device key stops working immediately. The device stays in this
            list for audit and can never be re-enabled.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={revoke.isPending}>
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={revoke.isPending}
            onClick={() => void handleRevoke()}
          >
            {revoke.isPending ? "Revoking..." : "Revoke device"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
