"use client"

import { ApiError } from "@workspace/api-client"

const messages: Record<string, string> = {
  FORBIDDEN: "Admin access required.",
  VALIDATION_ERROR: "That request is not valid. Refresh the page and try again.",
  RATE_LIMITED: "Too many requests. Please wait a moment and try again.",
}

const FALLBACK = "Could not load admin data. Please try again."

export function adminErrorMessage(error: unknown): string {
  if (error instanceof ApiError && messages[error.code]) {
    return messages[error.code]
  }
  return FALLBACK
}
