"use client"

import { ApiError } from "@workspace/api-client"

const messages: Record<string, string> = {
  PLAN_NOT_FOUND: "This plan is no longer available.",
  SUBSCRIPTION_NOT_FOUND: "You do not have an active subscription.",
  SUBSCRIPTION_ALREADY_ACTIVE:
    "You already have an active subscription. Upgrade or downgrade instead.",
  INVALID_STATUS_TRANSITION: "That action is not available right now.",
  VALIDATION_ERROR: "Please check the details and try again.",
  PLAN_LIMIT_REACHED:
    "You have reached your plan's limit. Upgrade to continue.",
  RATE_LIMITED: "Too many requests. Please try again shortly.",
}

const FALLBACK = "Something went wrong. Please try again."

export function billingErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return messages[error.code] ?? FALLBACK
  }
  return FALLBACK
}
