"use client"

import type { components } from "@workspace/api-client"
import { Button } from "@workspace/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@workspace/ui/components/dialog"

type Photo = components["schemas"]["Photo"]

export function DeletePhotoDialog({
  photo,
  pending,
  onOpenChange,
  onConfirm,
}: {
  photo: Photo | null
  pending: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
}) {
  return (
    <Dialog open={photo !== null} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete photo?</DialogTitle>
          <DialogDescription>
            &ldquo;{photo?.filename || "This photo"}&rdquo; will be removed from
            the event and its gallery. This cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            variant="outline"
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={pending}
            onClick={onConfirm}
          >
            {pending ? "Deleting..." : "Delete photo"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
