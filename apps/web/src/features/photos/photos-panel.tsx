"use client"

import * as React from "react"
import { toast } from "sonner"

import type { components } from "@workspace/api-client"
import { Card, CardContent } from "@workspace/ui/components/card"

import { useEvent } from "@/features/events/api"

import { useDeletePhoto, useEventPhotos } from "./api"
import { DeletePhotoDialog } from "./delete-photo-dialog"
import { photoErrorMessage } from "./errors"
import { PhotoGrid } from "./photo-grid"
import { UploadDropzone } from "./upload-dropzone"
import { useUploadItems, useUploadQueue } from "./upload-provider"
import { UploadQueueCard } from "./upload-queue"

type Photo = components["schemas"]["Photo"]

export function PhotosPanel({ eventId }: { eventId: string }) {
  const queue = useUploadQueue()
  const items = useUploadItems(queue)
  const deletePhoto = useDeletePhoto(eventId)
  const photos = useEventPhotos(eventId)
  const event = useEvent(eventId)
  const [pendingDelete, setPendingDelete] = React.useState<Photo[] | null>(null)

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
    if (!pendingDelete || pendingDelete.length === 0) return
    const targets = pendingDelete
    let deleted = 0
    const failures: unknown[] = []
    try {
      for (const photo of targets) {
        try {
          await deletePhoto.mutateAsync(photo.id)
          deleted += 1
        } catch (error) {
          failures.push(error)
        }
      }
      if (deleted > 0) {
        toast.success(
          deleted === 1 ? "Photo deleted" : `${deleted} photos deleted`
        )
      }
      if (failures.length > 0) {
        toast.error(
          failures.length === 1
            ? photoErrorMessage(failures[0])
            : `${failures.length} photos could not be deleted. Please try again.`
        )
      }
    } finally {
      setPendingDelete(null)
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardContent>
          <UploadDropzone
            onFiles={handleFiles}
            onDropError={(message) => toast.error(message)}
          />
        </CardContent>
      </Card>

      <UploadQueueCard
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
        onClearFinished={() => queue.clearFinished()}
      />

      <PhotoGrid
        eventId={eventId}
        coverPhotoId={event.data?.coverPhotoId ?? null}
        photoCount={event.data?.photoCount}
        onDelete={setPendingDelete}
      />

      <DeletePhotoDialog
        photos={pendingDelete ?? []}
        pending={deletePhoto.isPending}
        onOpenChange={(open) => {
          if (!open) setPendingDelete(null)
        }}
        onConfirm={() => void handleConfirmDelete()}
      />
    </div>
  )
}
