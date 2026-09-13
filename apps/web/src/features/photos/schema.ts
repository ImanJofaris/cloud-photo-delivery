export const ALLOWED_MIME_TYPES = [
  "image/jpeg",
  "image/png",
  "image/webp",
] as const

export const MAX_FILE_SIZE = 100 * 1024 * 1024
export const MULTIPART_FLOOR = 10 * 1024 * 1024

export type FileDescriptor = {
  name: string
  type: string
  size: number
}

export function validateFile(file: FileDescriptor): string | null {
  if (!file.name.trim()) {
    return "The file needs a name."
  }
  if (
    !ALLOWED_MIME_TYPES.includes(
      file.type as (typeof ALLOWED_MIME_TYPES)[number]
    )
  ) {
    return "Only JPEG, PNG, and WebP photos are supported."
  }
  if (file.size < 1) {
    return "The file is empty."
  }
  if (file.size > MAX_FILE_SIZE) {
    return "Photos must be 100 MB or smaller."
  }
  return null
}

export function uploadKindFor(size: number): "simple" | "multipart" {
  return size >= MULTIPART_FLOOR ? "multipart" : "simple"
}
