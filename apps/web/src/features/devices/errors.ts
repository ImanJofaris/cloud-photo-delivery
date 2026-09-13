"use client"

import { ApiError } from "@workspace/api-client"

const messages: Record<string, string> = {
  DEVICE_NOT_FOUND: "This device no longer exists.",
  DEVICE_REVOKED: "This device key has been revoked. Rotate the key first.",
  EVENT_NOT_FOUND: "The selected event no longer exists.",
  VALIDATION_ERROR: "Please check the form and try again.",
  RATE_LIMITED: "Too many requests. Please wait a moment and try again.",
}

const FALLBACK = "Something went wrong. Please try again."

export function deviceErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return messages[error.code] ?? FALLBACK
  }
  return FALLBACK
}
