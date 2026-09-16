"use client"

import * as React from "react"
import { toast } from "sonner"

import type { components } from "@workspace/api-client"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import { useDeletePhoto, useEventPhotos } from "./api"
import { DeletePhotoDialog } from "./delete-photo-dialog"
import { photoErrorMessage } from "./errors"
import { PhotoGrid } from "./photo-grid"
import { UploadDropzone } from "./upload-dropzone"
import { useUploadItems, useUploadQueue } from "./upload-provider"
import { UploadQueueList } from "./upload-queue"

type Photo = components["schemas"]["Photo"]

export function PhotosPanel({ eventId }: { eventId: string }) {
  const queue = useUploadQueue()
  const items = useUploadItems(queue)
  const deletePhoto = useDeletePhoto(eventId)
  const photos = useEventPhotos(eventId)
  const [pendingDelete, setPendingDelete] = React.useState<Photo | null>(null)

  const photoPages = photos.data?.pages
  React.useEffect(() => {
    if (!photoPages) return
    queue.applyPhotoStatuses(photoPages.flatMap((page) => page.items))
  }, [photoPages, queue])

  function handleFiles(files: File[]) {
    const rejected = queue.addFiles(files)
    for (const reject of rejected) {
      toast.error(`${reject.filename}: ${reject.reason}`)
    }
  }

  async function handleConfirmDelete() {
    if (!pendingDelete) return
    try {
      await deletePhoto.mutateAsync(pendingDelete.id)
      toast.success("Photo deleted")
    } catch (error) {
      toast.error(photoErrorMessage(error))
    } finally {
      setPendingDelete(null)
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Upload photos</CardTitle>
          <CardDescription>
            Files upload directly to storage. They appear in the gallery once
            processing finishes.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <UploadDropzone
            onFiles={handleFiles}
            onDropError={(message) => toast.error(message)}
          />
          <UploadQueueList
            items={items}
            onRetry={(key) => queue.retry(key)}
            onCancel={(key) => {
              void queue.cancel(key).catch((error: unknown) => {
                toast.error(photoErrorMessage(error))
              })
            }}
            onDiscard={(key) => {
              void queue.discard(key).catch((error: unknown) => {
                toast.error(photoErrorMessage(error))
              })
            }}
          />
        </CardContent>
      </Card>

      <PhotoGrid eventId={eventId} onDelete={setPendingDelete} />

      <DeletePhotoDialog
        photo={pendingDelete}
        pending={deletePhoto.isPending}
        onOpenChange={(open) => {
          if (!open) setPendingDelete(null)
        }}
        onConfirm={() => void handleConfirmDelete()}
      />
    </div>
  )
}
