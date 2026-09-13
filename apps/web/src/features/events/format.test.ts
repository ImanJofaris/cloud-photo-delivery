import { describe, expect, it } from "vitest"

import { formatBytes, formatDate, qrDownloadFilename } from "./format"

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
