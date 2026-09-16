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

export function extendEventErrorMessage(error: unknown): string {
  if (error instanceof ApiError && error.code === "INVALID_STATUS_TRANSITION") {
    return "Archived events cannot be extended."
  }
  return eventErrorMessage(error)
}

export function exportErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "EXPORT_NOT_FOUND") {
      return "That export is no longer available."
    }
    if (error.code === "VALIDATION_ERROR") {
      return "Add photos to this event before exporting."
    }
    if (error.code === "RATE_LIMITED") {
      return "Too many export requests. Please try again shortly."
    }
  }
  return eventErrorMessage(error)
}

export function isExportNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.code === "EXPORT_NOT_FOUND"
}
