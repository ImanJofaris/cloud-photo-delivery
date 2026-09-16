"use client"

import * as React from "react"
import { Download, FileArchive, LoaderCircle } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import { useCreateEventExport, useEventExport } from "./api"
import { exportErrorMessage, isExportNotFound } from "./errors"
import { formatBytes, formatTimeUntil } from "./format"

const STORAGE_PREFIX = "cpd:export:"

const storageListeners = new Set<() => void>()

function storageKey(eventId: string) {
  return `${STORAGE_PREFIX}${eventId}`
}

function sessionStorageOrNull(): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}

export function readStoredExportId(eventId: string): string | null {
  try {
    return sessionStorageOrNull()?.getItem(storageKey(eventId)) ?? null
  } catch {
    return null
  }
}

function storeExportId(eventId: string, exportId: string | null) {
  try {
    const storage = sessionStorageOrNull()
    if (exportId) {
      storage?.setItem(storageKey(eventId), exportId)
    } else {
      storage?.removeItem(storageKey(eventId))
    }
  } catch {
    // The export still polls in this tab; only reloads lose the id.
  }
  storageListeners.forEach((listener) => listener())
}

function subscribeExportId(listener: () => void) {
  storageListeners.add(listener)
  return () => {
    storageListeners.delete(listener)
  }
}

export function ExportCard({
  eventId,
  photoCount,
}: {
  eventId: string
  photoCount?: number
}) {
  const exportId = React.useSyncExternalStore(
    subscribeExportId,
    () => readStoredExportId(eventId),
    () => null
  )
  const exportQuery = useEventExport(eventId, exportId)
  const createExport = useCreateEventExport(eventId)
  const exportJob = exportQuery.data

  React.useEffect(() => {
    if (exportId && isExportNotFound(exportQuery.error)) {
      storeExportId(eventId, null)
    }
  }, [eventId, exportId, exportQuery.error])

  async function handleCreate() {
    try {
      const created = await createExport.mutateAsync()
      storeExportId(eventId, created.id)
      toast.success("Export started")
    } catch (error) {
      toast.error(exportErrorMessage(error))
    }
  }

  const notFound = exportId !== null && isExportNotFound(exportQuery.error)
  const activeExportId = notFound ? null : exportId
  const status = exportJob?.status
  const isWorking = status === "pending" || status === "processing"
  const remaining = exportJob ? formatTimeUntil(exportJob.expiresAt) : null
  const noPhotos = photoCount === 0

  return (
    <Card>
      <CardHeader>
        <CardTitle>Bulk export</CardTitle>
        <CardDescription>
          Download every photo in this event as a ZIP archive.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {!activeExportId && noPhotos && (
          <p className="text-sm text-muted-foreground">
            Add photos before generating an export.
          </p>
        )}

        {!activeExportId && (
          <Button
            onClick={() => void handleCreate()}
            disabled={noPhotos || createExport.isPending}
          >
            <FileArchive />
            {createExport.isPending ? "Starting..." : "Export all photos (ZIP)"}
          </Button>
        )}

        {activeExportId && (exportQuery.isPending || isWorking) && (
          <p
            role="status"
            className="flex items-center gap-2 text-sm text-muted-foreground"
          >
            <LoaderCircle className="size-4 animate-spin" />
            {exportQuery.isPending
              ? "Checking export status..."
              : "Preparing ZIP... This can take a few minutes."}
          </p>
        )}

        {status === "ready" && exportJob && (
          <div className="space-y-3">
            <p className="text-sm">
              Ready
              {exportJob.fileSize !== null
                ? ` · ${formatBytes(exportJob.fileSize)}`
                : ""}
              {remaining ? (
                <span className="text-muted-foreground">
                  {" · expires in "}
                  {remaining}
                </span>
              ) : null}
            </p>
            <div className="flex flex-wrap gap-2">
              {exportJob.downloadUrl ? (
                <Button
                  render={<a href={exportJob.downloadUrl} />}
                  nativeButton={false}
                >
                  <Download />
                  Download ZIP
                </Button>
              ) : (
                <Button disabled>
                  <Download />
                  Download ZIP
                </Button>
              )}
              <Button
                variant="outline"
                onClick={() => void handleCreate()}
                disabled={createExport.isPending}
              >
                Generate new export
              </Button>
            </div>
          </div>
        )}

        {status === "failed" && (
          <div className="space-y-3">
            <p className="text-sm text-destructive">
              The export could not be generated.
            </p>
            <Button
              variant="outline"
              onClick={() => void handleCreate()}
              disabled={createExport.isPending}
            >
              Generate new export
            </Button>
          </div>
        )}

        {status === "expired" && (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              This export has expired. Generate a new one to download it again.
            </p>
            <Button
              variant="outline"
              onClick={() => void handleCreate()}
              disabled={createExport.isPending}
            >
              Generate new export
            </Button>
          </div>
        )}

        {activeExportId &&
          exportQuery.isError &&
          !isExportNotFound(exportQuery.error) && (
            <div className="space-y-3">
              <p className="text-sm text-destructive">
                {exportErrorMessage(exportQuery.error)}
              </p>
              <Button
                variant="outline"
                onClick={() => void exportQuery.refetch()}
              >
                Try again
              </Button>
            </div>
          )}
      </CardContent>
    </Card>
  )
}
