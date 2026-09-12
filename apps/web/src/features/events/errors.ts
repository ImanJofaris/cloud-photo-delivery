"use client"

import { ApiError } from "@workspace/api-client"

const messages: Record<string, string> = {
  EVENT_NOT_FOUND: "This event no longer exists.",
  INVALID_STATUS_TRANSITION:
    "That action is not allowed for the event's current status.",
  PLAN_LIMIT_REACHED: "You have reached your plan's active event limit.",
  VALIDATION_ERROR: "Please check the form and try again.",
}

const FALLBACK = "Something went wrong. Please try again."

export function eventErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (messages[error.code]) {
      return messages[error.code]
    }
    if (error.status === 402) {
      return messages.PLAN_LIMIT_REACHED
    }
  }
  return FALLBACK
}
