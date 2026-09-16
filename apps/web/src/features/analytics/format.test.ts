import { describe, expect, it } from "vitest"

import type { components } from "@workspace/api-client"

import {
  fillDailySeries,
  formatCount,
  formatUtcDay,
  formatUtcDayLong,
  seriesTotal,
  utcDaySeries,
} from "./format"

type AnalyticsDay = components["schemas"]["AnalyticsDay"]

function day(overrides: Partial<AnalyticsDay> = {}): AnalyticsDay {
  return {
    date: "2026-09-12",
    galleryViews: 0,
    uniqueVisitors: 0,
    downloads: 0,
    qrScans: 0,
    ...overrides,
  }
}

describe("formatCount", () => {
  it("formats numbers and blanks", () => {
    expect(formatCount(1234)).toBe("1,234")
    expect(formatCount(0)).toBe("0")
    expect(formatCount(null)).toBe("—")
    expect(formatCount(undefined)).toBe("—")
  })
})

describe("utcDaySeries", () => {
  it("builds a contiguous UTC window ending today", () => {
    const series = utcDaySeries(3, new Date("2026-09-16T23:30:00Z"))
    expect(series).toEqual(["2026-09-14", "2026-09-15", "2026-09-16"])
  })

  it("spans month boundaries", () => {
    const series = utcDaySeries(3, new Date("2026-09-01T00:30:00Z"))
    expect(series).toEqual(["2026-08-30", "2026-08-31", "2026-09-01"])
  })
})

describe("fillDailySeries", () => {
  it("zero-fills missing days inside the window", () => {
    const filled = fillDailySeries(
      [day({ date: "2026-09-14", galleryViews: 4 })],
      3,
      new Date("2026-09-16T12:00:00Z")
    )

    expect(filled.map((entry) => entry.date)).toEqual([
      "2026-09-14",
      "2026-09-15",
      "2026-09-16",
    ])
    expect(filled[0].galleryViews).toBe(4)
    expect(filled[1].galleryViews).toBe(0)
    expect(filled[2].qrScans).toBe(0)
  })

  it("keeps days outside the window out of the series", () => {
    const filled = fillDailySeries(
      [day({ date: "2026-01-01", downloads: 9 })],
      2,
      new Date("2026-09-16T12:00:00Z")
    )

    expect(filled).toHaveLength(2)
    expect(seriesTotal(filled, "downloads")).toBe(0)
  })
})

describe("UTC day labels", () => {
  it("formats short and long UTC dates", () => {
    expect(formatUtcDay("2026-09-12")).toMatch(/Sep/)
    expect(formatUtcDay("2026-09-12")).toMatch(/12/)
    expect(formatUtcDayLong("2026-09-12")).toMatch(/2026/)
  })

  it("handles invalid input", () => {
    expect(formatUtcDay("nope")).toBe("—")
    expect(formatUtcDayLong("nope")).toBe("—")
  })
})

describe("seriesTotal", () => {
  it("sums the selected metric", () => {
    const filled = [
      day({ galleryViews: 2, qrScans: 1 }),
      day({ galleryViews: 3, qrScans: 4 }),
    ]

    expect(seriesTotal(filled, "galleryViews")).toBe(5)
    expect(seriesTotal(filled, "qrScans")).toBe(5)
  })
})
