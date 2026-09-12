import { ApiError } from "@workspace/api-client"

const messages: Record<string, string> = {
  INVALID_CREDENTIALS: "Incorrect email or password.",
  ACCOUNT_LOCKED:
    "Too many failed attempts. Your account is locked for a while.",
  EMAIL_ALREADY_REGISTERED: "An account with this email already exists.",
  VALIDATION_ERROR: "Please check the form and try again.",
  UNAUTHENTICATED: "Your session has expired. Please log in again.",
  RATE_LIMITED: "Too many requests. Please slow down and try again.",
  INVALID_RESET_TOKEN: "This reset link is invalid or has expired.",
}

const FALLBACK = "Something went wrong. Please try again."

export function authErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (messages[error.code]) {
      return messages[error.code]
    }
    if (error.status === 429) {
      return messages.RATE_LIMITED
    }
    if (error.status === 423) {
      return messages.ACCOUNT_LOCKED
    }
  }
  return FALLBACK
}

export { FALLBACK as AUTH_ERROR_FALLBACK }
