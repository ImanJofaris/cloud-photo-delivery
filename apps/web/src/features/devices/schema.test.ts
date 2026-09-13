import { describe, expect, it } from "vitest"

import { createDeviceSchema, toCreateDeviceRequest } from "./schema"

describe("createDeviceSchema", () => {
  it("requires a non-blank name", () => {
    const result = createDeviceSchema.safeParse({
      name: "   ",
      assignedEventId: "no-event",
    })
    expect(result.success).toBe(false)
  })

  it("rejects names over 120 characters", () => {
    const result = createDeviceSchema.safeParse({
      name: "a".repeat(121),
      assignedEventId: "no-event",
    })
    expect(result.success).toBe(false)
  })
})

describe("toCreateDeviceRequest", () => {
  it("omits the assigned event when unscoped", () => {
    expect(
      toCreateDeviceRequest({ name: "  Booth 1  ", assignedEventId: "no-event" })
    ).toEqual({ name: "Booth 1" })
  })

  it("includes the assigned event id when selected", () => {
    expect(
      toCreateDeviceRequest({
        name: "Booth 1",
        assignedEventId: "event-1",
      })
    ).toEqual({ name: "Booth 1", assignedEventId: "event-1" })
  })
})
