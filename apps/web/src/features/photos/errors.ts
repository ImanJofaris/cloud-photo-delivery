"use client"

import { ApiError } from "@workspace/api-client"

const messages: Record<string, string> = {
  EVENT_NOT_FOUND: "This event no longer exists.",
  PHOTO_NOT_FOUND: "This photo no longer exists.",
  UPLOAD_NOT_FOUND: "The upload could not be found. Please try again.",
  UPLOAD_SIZE_MISMATCH:
    "The uploaded file did not match its expected size. Please try again.",
  IDEMPOTENCY_CONFLICT: "This upload conflicts with an earlier request.",
  NOT_SIMPLE: "This upload must be resumed as a multipart upload.",
  INVALID_UPLOAD_STATE: "This upload has already finished.",
  VALIDATION_ERROR: "The server rejected this file.",
}

const FALLBACK = "Something went wrong. Please try again."

export function photoErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return messages[error.code] ?? error.message ?? FALLBACK
  }
  if (error instanceof Error && error.message) {
    return error.message
  }
  return FALLBACK
}

export function isNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404
}
