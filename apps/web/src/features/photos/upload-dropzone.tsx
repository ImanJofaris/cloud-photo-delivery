"use client"

import { ImagePlus } from "lucide-react"
import * as React from "react"

import { Button } from "@workspace/ui/components/button"
import { cn } from "@workspace/ui/lib/utils"

import { ALLOWED_MIME_TYPES } from "./schema"

const PHOTO_NAME_PATTERN = /\.(jpe?g|png|webp)$/i

function isPhotoLike(file: File): boolean {
  return (
    ALLOWED_MIME_TYPES.includes(
      file.type as (typeof ALLOWED_MIME_TYPES)[number]
    ) || PHOTO_NAME_PATTERN.test(file.name)
  )
}

function hasFiles(transfer: DataTransfer | null): boolean {
  return Boolean(transfer && Array.from(transfer.types).includes("Files"))
}

function readFileEntry(entry: FileSystemFileEntry): Promise<File> {
  return new Promise((resolve, reject) => {
    entry.file(resolve, reject)
  })
}

function readEntries(
  reader: FileSystemDirectoryReader
): Promise<FileSystemEntry[]> {
  return new Promise((resolve, reject) => {
    reader.readEntries(resolve, reject)
  })
}

async function readAllEntries(
  reader: FileSystemDirectoryReader
): Promise<FileSystemEntry[]> {
  const entries: FileSystemEntry[] = []
  for (;;) {
    const batch = await readEntries(reader)
    if (batch.length === 0) return entries
    entries.push(...batch)
  }
}

async function filesFromEntry(entry: FileSystemEntry): Promise<File[]> {
  if (entry.isFile) {
    return [await readFileEntry(entry as FileSystemFileEntry)]
  }
  if (entry.isDirectory) {
    const reader = (entry as FileSystemDirectoryEntry).createReader()
    const children = await readAllEntries(reader)
    const nested = await Promise.all(children.map(filesFromEntry))
    return nested.flat()
  }
  return []
}

function entryFor(item: DataTransferItem): FileSystemEntry | null {
  const legacy = item as DataTransferItem & {
    webkitGetAsEntry?: () => FileSystemEntry | null
  }
  return legacy.webkitGetAsEntry?.() ?? null
}

export function UploadDropzone({
  onFiles,
  disabled,
  onDropError,
}: {
  onFiles: (files: File[]) => void
  disabled?: boolean
  onDropError?: (message: string) => void
}) {
  const inputRef = React.useRef<HTMLInputElement>(null)
  const dragDepth = React.useRef(0)
  const [dragging, setDragging] = React.useState(false)

  React.useEffect(() => {
    function block(event: DragEvent) {
      if (hasFiles(event.dataTransfer)) event.preventDefault()
    }
    window.addEventListener("dragover", block)
    window.addEventListener("drop", block)
    return () => {
      window.removeEventListener("dragover", block)
      window.removeEventListener("drop", block)
    }
  }, [])

  function handleFiles(list: FileList | File[] | null) {
    if (!list) return
    const files = Array.from(list)
    if (files.length === 0) return
    onFiles(files)
  }

  async function handleDrop(event: React.DragEvent<HTMLDivElement>) {
    event.preventDefault()
    dragDepth.current = 0
    setDragging(false)
    if (disabled) return

    const transfer = event.dataTransfer
    if (transfer.files.length > 0) {
      handleFiles(transfer.files)
      return
    }

    const entries = Array.from(transfer.items)
      .map(entryFor)
      .filter((entry): entry is FileSystemEntry => entry !== null)

    if (entries.length === 0) {
      onDropError?.(
        "That drop did not contain any files. Drag photos from a folder window instead."
      )
      return
    }

    let photos: File[]
    try {
      const files = (await Promise.all(entries.map(filesFromEntry))).flat()
      photos = files.filter(isPhotoLike)
    } catch {
      onDropError?.(
        "The dropped files could not be read. Drag them from a folder window instead."
      )
      return
    }
    if (photos.length === 0) {
      onDropError?.("No JPEG, PNG, or WebP photos were found in that drop.")
      return
    }
    onFiles(photos)
  }

  return (
    <div
      data-testid="upload-dropzone"
      onDragEnter={(event) => {
        event.preventDefault()
        dragDepth.current += 1
        if (!disabled) setDragging(true)
      }}
      onDragOver={(event) => {
        event.preventDefault()
        if (event.dataTransfer) {
          event.dataTransfer.dropEffect = disabled ? "none" : "copy"
        }
      }}
      onDragLeave={() => {
        dragDepth.current = Math.max(0, dragDepth.current - 1)
        if (dragDepth.current === 0) setDragging(false)
      }}
      onDrop={(event) => void handleDrop(event)}
      className={cn(
        "flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed p-6 text-center transition-colors",
        dragging && "border-primary bg-primary/5"
      )}
    >
      <ImagePlus className="size-6 text-muted-foreground" />
      <p className="text-sm text-muted-foreground">Drag photos here, or</p>
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => inputRef.current?.click()}
      >
        Choose photos
      </Button>
      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        multiple
        className="hidden"
        onChange={(event) => {
          handleFiles(event.target.files)
          event.target.value = ""
        }}
      />
      <p className="text-xs text-muted-foreground">
        You can drag a whole folder too · JPEG, PNG or WebP · up to 100 MB each
      </p>
    </div>
  )
}
