import { describe, expect, it } from "vitest"

import {
  expiryFromNow,
  formatBytes,
  formatDate,
  formatTimeUntil,
  qrDownloadFilename,
} from "./format"

describe("formatBytes", () => {
  it("formats zero and small values", () => {
    expect(formatBytes(0)).toBe("0 B")
    expect(formatBytes(512)).toBe("512 B")
  })

  it("formats kilobytes, megabytes, and gigabytes", () => {
    expect(formatBytes(2048)).toBe("2.0 KB")
    expect(formatBytes(1024 * 1024 * 12)).toBe("12 MB")
    expect(formatBytes(1024 * 1024 * 1024 * 8.4)).toBe("8.4 GB")
  })
})

describe("formatDate", () => {
  it("formats an ISO date", () => {
    expect(formatDate("2026-09-12")).toContain("2026")
  })

  it("returns a placeholder for empty values", () => {
    expect(formatDate(null)).toBe("—")
    expect(formatDate("not-a-date")).toBe("—")
  })
})

describe("qrDownloadFilename", () => {
  it("slugifies the event name", () => {
    expect(qrDownloadFilename("Sarah & John's Wedding", "png")).toBe(
      "sarah-john-s-wedding-qr.png"
    )
    expect(qrDownloadFilename("  Wedding  ", "svg")).toBe("wedding-qr.svg")
  })

  it("falls back when the name has no usable characters", () => {
    expect(qrDownloadFilename("***", "png")).toBe("event-qr.png")
  })
})

const now = Date.parse("2026-09-16T12:00:00Z")

describe("expiryFromNow", () => {
  it("warns at exactly seven days", () => {
    expect(expiryFromNow("2026-09-23T12:00:00Z", now)).toEqual({
      tone: "warning",
      label: "Expires in 7 days",
    })
  })

  it("stays neutral beyond the warning window", () => {
    expect(expiryFromNow("2026-09-24T12:00:00Z", now)).toEqual({
      tone: "neutral",
      label: "Expires in 8 days",
    })
  })

  it("labels a same-day expiry", () => {
    expect(expiryFromNow("2026-09-16T20:00:00Z", now)).toEqual({
      tone: "warning",
      label: "Expires today",
    })
  })

  it("labels a past expiry", () => {
    expect(expiryFromNow("2026-09-14T12:00:00Z", now)).toEqual({
      tone: "expired",
      label: "Expired 2 days ago",
    })
  })

  it("labels never-expiring and invalid values", () => {
    expect(expiryFromNow(null, now)).toEqual({
      tone: "neutral",
      label: "Never expires",
    })
    expect(expiryFromNow("not-a-date", now).label).toBe("Never expires")
  })
})

describe("formatTimeUntil", () => {
  it("formats minutes, hours, and days", () => {
    expect(formatTimeUntil("2026-09-16T12:30:00Z", now)).toBe("30 minutes")
    expect(formatTimeUntil("2026-09-16T13:00:00Z", now)).toBe("1 hour")
    expect(formatTimeUntil("2026-09-18T12:00:00Z", now)).toBe("2 days")
  })

  it("returns null for past or missing values", () => {
    expect(formatTimeUntil("2026-09-16T11:00:00Z", now)).toBeNull()
    expect(formatTimeUntil(null, now)).toBeNull()
  })
})
