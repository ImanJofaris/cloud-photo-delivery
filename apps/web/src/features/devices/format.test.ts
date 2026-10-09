import { describe, expect, it } from "vitest"

import {
  DEVICE_FRESH_WINDOW_MS,
  deviceFreshness,
  formatDeviceTimestamp,
  formatRelativeTime,
  resolveEventName,
  UNKNOWN_EVENT,
} from "./format"

const NOW = Date.UTC(2026, 8, 13, 12, 0, 0)

describe("formatRelativeTime", () => {
  it("renders Just now for very recent timestamps", () => {
    expect(formatRelativeTime("2026-09-13T11:59:30Z", NOW)).toBe("Just now")
  })

  it("renders minutes and hours", () => {
    expect(formatRelativeTime("2026-09-13T11:58:00Z", NOW)).toBe("2 min ago")
    expect(formatRelativeTime("2026-09-13T11:00:00Z", NOW)).toBe("1 hour ago")
    expect(formatRelativeTime("2026-09-13T09:00:00Z", NOW)).toBe("3 hours ago")
  })

  it("falls back to a calendar date after 48 hours", () => {
    const label = formatRelativeTime("2026-09-11T11:00:00Z", NOW)
    expect(label).toContain("2026")
    expect(label).not.toContain("ago")
  })

  it("renders Never for null or invalid timestamps", () => {
    expect(formatRelativeTime(null, NOW)).toBe("Never")
    expect(formatRelativeTime("not-a-date", NOW)).toBe("Never")
  })
})

describe("deviceFreshness", () => {
  it("marks uploads inside the fresh window as fresh", () => {
    expect(
      deviceFreshness(new Date(NOW - 60_000).toISOString(), NOW)
    ).toBe("fresh")
    expect(
      deviceFreshness(new Date(NOW - DEVICE_FRESH_WINDOW_MS).toISOString(), NOW)
    ).toBe("fresh")
  })

  it("marks older uploads as stale", () => {
    expect(
      deviceFreshness(new Date(NOW - DEVICE_FRESH_WINDOW_MS - 1).toISOString(), NOW)
    ).toBe("stale")
  })

  it("marks missing uploads as never", () => {
    expect(deviceFreshness(null, NOW)).toBe("never")
    expect(deviceFreshness("not-a-date", NOW)).toBe("never")
  })
})

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
