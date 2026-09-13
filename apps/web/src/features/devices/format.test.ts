import { describe, expect, it } from "vitest"

import {
  formatDeviceTimestamp,
  resolveEventName,
  UNKNOWN_EVENT,
} from "./format"

describe("formatDeviceTimestamp", () => {
  it("formats a timestamp", () => {
    expect(formatDeviceTimestamp("2026-09-13T10:00:00Z")).not.toBe("Never")
  })

  it("renders Never for null", () => {
    expect(formatDeviceTimestamp(null)).toBe("Never")
  })
})

describe("resolveEventName", () => {
  const names = new Map([["event-1", "Sarah & John's Wedding"]])

  it("returns null for unscoped devices", () => {
    expect(resolveEventName(null, names)).toBeNull()
  })

  it("resolves a known event", () => {
    expect(resolveEventName("event-1", names)).toBe(
      "Sarah & John's Wedding"
    )
  })

  it("falls back for unknown events", () => {
    expect(resolveEventName("event-2", names)).toBe(UNKNOWN_EVENT)
  })
})
