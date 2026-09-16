"use client"

import { ApiError } from "@workspace/api-client"

const messages: Record<string, string> = {
  EVENT_NOT_FOUND: "This event no longer exists.",
  VALIDATION_ERROR: "That analytics window is not valid. Use 1 to 365 days.",
  RATE_LIMITED: "Too many requests. Please wait a moment and try again.",
}

const FALLBACK = "Could not load analytics. Please try again."

export function analyticsErrorMessage(error: unknown): string {
  if (error instanceof ApiError && messages[error.code]) {
    return messages[error.code]
  }
  return FALLBACK
}
