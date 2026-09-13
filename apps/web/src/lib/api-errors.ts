import { ApiError } from "@workspace/api-client"

export function isPlanLimitReached(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    (error.code === "PLAN_LIMIT_REACHED" || error.status === 402)
  )
}
