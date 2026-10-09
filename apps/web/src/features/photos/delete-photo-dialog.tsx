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
  photos,
  pending,
  onOpenChange,
  onConfirm,
}: {
  photos: Photo[]
  pending: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
}) {
  const single = photos.length === 1 ? photos[0] : null
  return (
    <Dialog open={photos.length > 0} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {single ? "Delete photo?" : `Delete ${photos.length} photos?`}
          </DialogTitle>
          <DialogDescription>
            {single
              ? `"${single.filename || "This photo"}" will be removed from the event and its gallery. This cannot be undone.`
              : `These ${photos.length} photos will be removed from the event and its gallery. This cannot be undone.`}
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
            {pending
              ? "Deleting..."
              : single
                ? "Delete photo"
                : `Delete ${photos.length} photos`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
