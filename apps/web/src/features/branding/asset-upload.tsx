"use client"

import { ImagePlus } from "lucide-react"
import * as React from "react"

import { ApiError, type components } from "@workspace/api-client"

import { Button } from "@workspace/ui/components/button"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"
import { Progress } from "@workspace/ui/components/progress"

import {
  UploadTransportError,
  putWithProgress,
  type PutTransport,
} from "@/features/photos/transport"

import { useCreateBrandingAssetUpload } from "./api"
import {
  validateBrandingAsset,
  type BrandingAssetKind,
} from "./schema"

type BrandingAsset = components["schemas"]["BrandingAsset"]
type BrandingAssetRequest = components["schemas"]["BrandingAssetRequest"]

export function isExpiredUrlError(error: unknown): boolean {
  return (
    error instanceof UploadTransportError &&
    (error.status === 400 || error.status === 403)
  )
}

export async function uploadBrandingAsset({
  file,
  kind,
  presign,
  put,
  onProgress,
}: {
  file: File
  kind: BrandingAssetKind
  presign: (request: BrandingAssetRequest) => Promise<BrandingAsset>
  put: PutTransport
  onProgress?: (percent: number) => void
}): Promise<{ storageKey: string }> {
  const request: BrandingAssetRequest = {
    kind,
    contentType: file.type as BrandingAssetRequest["contentType"],
  }

  const attempt = async (): Promise<BrandingAsset> => {
    const asset = await presign(request)
    await put({
      url: asset.uploadUrl,
      body: file,
      onProgress: (loaded, total) => {
        onProgress?.(
          total > 0 ? Math.min(100, Math.round((loaded / total) * 100)) : 0
        )
      },
    })
    return asset
  }

  let asset: BrandingAsset
  try {
    asset = await attempt()
  } catch (error) {
    if (!isExpiredUrlError(error)) throw error
    asset = await attempt()
  }

  return { storageKey: asset.storageKey }
}

function uploadErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return "The upload could not be started. Please try again."
  }
  if (error instanceof UploadTransportError) {
    return "The upload failed. Check your connection and try again."
  }
  return "The upload failed. Please try again."
}

export function AssetUploadField({
  kind,
  label,
  description,
  previewUrl,
  hasAsset,
  onUploaded,
  onClear,
}: {
  kind: BrandingAssetKind
  label: string
  description: string
  previewUrl: string | null
  hasAsset: boolean
  onUploaded: (result: { storageKey: string; file: File }) => void
  onClear: () => void
}) {
  const presign = useCreateBrandingAssetUpload()
  const inputRef = React.useRef<HTMLInputElement>(null)
  const [progress, setProgress] = React.useState(0)
  const [uploading, setUploading] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [lastFile, setLastFile] = React.useState<File | null>(null)

  async function upload(file: File) {
    const reason = validateBrandingAsset(file)
    if (reason) {
      setError(reason)
      return
    }
    setLastFile(file)
    setError(null)
    setProgress(0)
    setUploading(true)
    try {
      const result = await uploadBrandingAsset({
        file,
        kind,
        presign: (request) => presign.mutateAsync(request),
        put: putWithProgress,
        onProgress: setProgress,
      })
      onUploaded({ storageKey: result.storageKey, file })
    } catch (uploadError) {
      setError(uploadErrorMessage(uploadError))
    } finally {
      setUploading(false)
    }
  }

  function handleChange(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ""
    if (file) void upload(file)
  }

  return (
    <div className="space-y-2">
      <Label htmlFor={`branding-${kind}`}>{label}</Label>
      <div className="flex items-center gap-3">
        <div className="flex size-14 shrink-0 items-center justify-center overflow-hidden rounded-md border bg-muted">
          {previewUrl ? (
            // Signed URLs and object URLs change per mount; next/image adds
            // nothing here.
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={previewUrl}
              alt=""
              className="size-full object-cover"
              decoding="async"
            />
          ) : (
            <ImagePlus className="size-5 text-muted-foreground" />
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={uploading}
            onClick={() => inputRef.current?.click()}
          >
            {uploading
              ? `Uploading ${progress}%`
              : hasAsset
                ? "Replace"
                : "Upload"}
          </Button>
          {hasAsset && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={uploading}
              onClick={onClear}
            >
              Remove
            </Button>
          )}
        </div>
      </div>
      <Input
        ref={inputRef}
        id={`branding-${kind}`}
        type="file"
        accept="image/png,image/jpeg,image/webp"
        className="sr-only"
        onChange={handleChange}
      />
      <p className="text-xs text-muted-foreground">{description}</p>
      {uploading && <Progress value={progress} />}
      {error && (
        <div className="flex items-center gap-2">
          <p className="text-xs text-destructive" role="alert">
            {error}
          </p>
          {lastFile && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => void upload(lastFile)}
            >
              Retry
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
