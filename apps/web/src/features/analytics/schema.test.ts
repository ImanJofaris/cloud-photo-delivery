import { describe, expect, it } from "vitest"

import { ApiError } from "@workspace/api-client"

import { analyticsErrorMessage } from "./errors"
import {
  ANALYTICS_MAX_DAYS,
  ANALYTICS_MIN_DAYS,
  ANALYTICS_WINDOWS,
  DEFAULT_ANALYTICS_WINDOW,
  parseAnalyticsDays,
} from "./schema"

describe("analytics window schema", () => {
  it("offers the four preset windows with 30 as the default", () => {
    expect(ANALYTICS_WINDOWS).toEqual([7, 30, 90, 365])
    expect(DEFAULT_ANALYTICS_WINDOW).toBe(30)
  })

  it("accepts whole days from 1 to 365", () => {
    expect(parseAnalyticsDays(ANALYTICS_MIN_DAYS)).toBe(1)
    expect(parseAnalyticsDays(365)).toBe(ANALYTICS_MAX_DAYS)
    expect(parseAnalyticsDays("90")).toBe(90)
  })

  it("falls back to the default outside the range", () => {
    expect(parseAnalyticsDays(0)).toBe(DEFAULT_ANALYTICS_WINDOW)
    expect(parseAnalyticsDays(366)).toBe(DEFAULT_ANALYTICS_WINDOW)
    expect(parseAnalyticsDays(30.5)).toBe(DEFAULT_ANALYTICS_WINDOW)
    expect(parseAnalyticsDays("nope")).toBe(DEFAULT_ANALYTICS_WINDOW)
    expect(parseAnalyticsDays(null)).toBe(DEFAULT_ANALYTICS_WINDOW)
  })
})

describe("analyticsErrorMessage", () => {
  it("maps known codes to copy", () => {
    expect(
      analyticsErrorMessage(new ApiError("EVENT_NOT_FOUND", "raw", 404))
    ).toMatch(/no longer exists/i)
    expect(
      analyticsErrorMessage(new ApiError("VALIDATION_ERROR", "raw", 422))
    ).toMatch(/1 to 365/)
    expect(
      analyticsErrorMessage(new ApiError("RATE_LIMITED", "raw", 429))
    ).toMatch(/too many requests/i)
  })

  it("never leaks the raw server message", () => {
    const message = analyticsErrorMessage(
      new ApiError("UNKNOWN_CODE", "raw server detail", 500)
    )
    expect(message).not.toContain("raw server detail")
  })
})
