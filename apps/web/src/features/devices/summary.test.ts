import { describe, expect, it } from "vitest"

import { summarizeDevices, type DeviceSummaryInput } from "./summary"

const base: DeviceSummaryInput = {
  name: "Booth",
  assignedEventId: null,
  revokedAt: null,
  lastUsedAt: null,
}

describe("summarizeDevices", () => {
  it("counts active and revoked devices", () => {
    const summary = summarizeDevices([
      base,
      { ...base, name: "Old iPad", revokedAt: "2026-09-13T11:00:00Z" },
    ])

    expect(summary.activeCount).toBe(1)
    expect(summary.revokedCount).toBe(1)
  })

  it("counts distinct events across active devices only", () => {
    const summary = summarizeDevices([
      { ...base, assignedEventId: "event-1" },
      { ...base, assignedEventId: "event-2" },
      { ...base, assignedEventId: "event-2" },
      {
        ...base,
        name: "Scoped but revoked",
        assignedEventId: "event-3",
        revokedAt: "2026-09-13T11:00:00Z",
      },
      base,
    ])

    expect(summary.scopedEventCount).toBe(2)
  })

  it("picks the most recent last upload among active devices", () => {
    const summary = summarizeDevices([
      { ...base, name: "Booth 1", lastUsedAt: "2026-09-13T10:00:00Z" },
      { ...base, name: "Booth 2", lastUsedAt: "2026-09-13T11:00:00Z" },
      {
        ...base,
        name: "Revoked",
        lastUsedAt: "2026-09-13T12:00:00Z",
        revokedAt: "2026-09-13T12:30:00Z",
      },
    ])

    expect(summary.lastUpload).toEqual({
      deviceName: "Booth 2",
      lastUsedAt: "2026-09-13T11:00:00Z",
    })
  })

  it("returns no last upload when active devices never uploaded", () => {
    const summary = summarizeDevices([
      { ...base, name: "Broken", lastUsedAt: "not-a-date" },
      base,
    ])

    expect(summary.lastUpload).toBeNull()
  })
})
