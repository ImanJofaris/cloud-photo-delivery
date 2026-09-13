"use client"

import { ApiError } from "@workspace/api-client"

const messages: Record<string, string> = {
  UNAUTHORIZED: "That password is not correct. Please try again.",
  EVENT_NOT_FOUND: "This gallery is no longer available.",
  PHOTO_NOT_FOUND: "This photo is no longer available.",
  VALIDATION_ERROR: "Please check the form and try again.",
}

const FALLBACK = "Something went wrong. Please try again."

export function isUnauthorized(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401
}

export function isNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404
}

export function galleryErrorMessage(error: unknown): string {
  if (error instanceof ApiError && messages[error.code]) {
    return messages[error.code]
  }
  return FALLBACK
}
