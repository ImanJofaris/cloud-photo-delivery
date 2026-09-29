import { describe, expect, it } from "vitest"

import { ApiError } from "@workspace/api-client"

import { retryUrls } from "./api"

describe("retryUrls", () => {
  it("retries 429 up to the retry limit", () => {
    const error = new ApiError("RATE_LIMITED", "slow down", 429)
    expect(retryUrls(0, error)).toBe(true)
    expect(retryUrls(3, error)).toBe(true)
    expect(retryUrls(4, error)).toBe(false)
  })

  it("does not retry permanent client errors", () => {
    expect(retryUrls(0, new ApiError("VARIANT_UNAVAILABLE", "no", 404))).toBe(
      false
    )
    expect(retryUrls(0, new ApiError("VALIDATION_ERROR", "bad", 422))).toBe(
      false
    )
    expect(retryUrls(0, new ApiError("UNAUTHORIZED", "no", 401))).toBe(false)
  })

  it("retries server and network errors once", () => {
    const server = new ApiError("INTERNAL_ERROR", "boom", 500)
    expect(retryUrls(0, server)).toBe(true)
    expect(retryUrls(1, server)).toBe(false)
    expect(retryUrls(0, new Error("network"))).toBe(true)
    expect(retryUrls(1, new Error("network"))).toBe(false)
  })
})
