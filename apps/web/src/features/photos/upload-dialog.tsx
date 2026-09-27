"use client"

import { Upload } from "lucide-react"
import * as React from "react"

import { Button } from "@workspace/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@workspace/ui/components/dialog"

import { UploadDropzone } from "./upload-dropzone"

export function UploadDialog({
  onFiles,
  onDropError,
}: {
  onFiles: (files: File[]) => void
  onDropError?: (message: string) => void
}) {
  const [open, setOpen] = React.useState(false)

  function handleFiles(files: File[]) {
    onFiles(files)
    setOpen(false)
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button type="button" data-testid="upload-photos-trigger">
            <Upload />
            Upload photos
          </Button>
        }
      />
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Upload photos</DialogTitle>
          <DialogDescription>
            Files upload directly to storage. They appear in the gallery once
            processing finishes.
          </DialogDescription>
        </DialogHeader>
        <UploadDropzone
          onFiles={handleFiles}
          onDropError={onDropError}
          className="min-h-80 sm:min-h-96"
        />
      </DialogContent>
    </Dialog>
  )
}
