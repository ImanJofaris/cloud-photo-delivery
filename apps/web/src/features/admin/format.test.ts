import { describe, expect, it } from "vitest"

import { formatMYR } from "@/features/billing/format"

import { formatBytes, formatPendingAge } from "./format"

describe("formatBytes", () => {
  it("formats byte counts reused from the events feature", () => {
    expect(formatBytes(0)).toBe("0 B")
    expect(formatBytes(3)).toBe("3 B")
    expect(formatBytes(1536)).toBe("1.5 KB")
    expect(formatBytes(53687091200)).toContain("GB")
  })
})

describe("formatMYR", () => {
  it("formats cents as ringgit", () => {
    expect(formatMYR(4900)).toMatch(/49[.,]00/)
    expect(formatMYR(0)).toMatch(/0[.,]00/)
  })
})

describe("formatPendingAge", () => {
  const now = new Date("2026-09-16T12:00:00Z").getTime()

  it("returns a dash for missing or invalid values", () => {
    expect(formatPendingAge(null, now)).toBe("—")
    expect(formatPendingAge(undefined, now)).toBe("—")
    expect(formatPendingAge("not-a-date", now)).toBe("—")
  })

  it("describes sub-minute ages as just now", () => {
    expect(formatPendingAge("2026-09-16T11:59:30Z", now)).toBe("just now")
  })

  it("formats minutes, hours, and days", () => {
    expect(formatPendingAge("2026-09-16T11:45:00Z", now)).toBe("15 minutes ago")
    expect(formatPendingAge("2026-09-16T11:00:00Z", now)).toBe("1 hour ago")
    expect(formatPendingAge("2026-09-16T06:00:00Z", now)).toBe("6 hours ago")
    expect(formatPendingAge("2026-09-14T12:00:00Z", now)).toBe("2 days ago")
  })
})
