import { describe, expect, it } from "vitest"

import { toCreateEventRequest, toUpdateEventSettingsRequest } from "./api"
import { createEventSchema, eventSettingsSchema } from "./schema"

describe("createEventSchema", () => {
  it("requires a name", () => {
    const result = createEventSchema.safeParse({ name: "   " })
    expect(result.success).toBe(false)
  })

  it("accepts a minimal event and trims the name", () => {
    const result = createEventSchema.parse({ name: "  Wedding  " })
    expect(result.name).toBe("Wedding")
  })

  it("rejects an invalid client email", () => {
    const result = createEventSchema.safeParse({
      name: "Wedding",
      clientEmail: "not-an-email",
    })
    expect(result.success).toBe(false)
  })
})

describe("eventSettingsSchema", () => {
  const base = {
    visibility: "public" as const,
    password: "",
    allowDownload: false,
    allowOriginalDownload: false,
    watermarkEnabled: false,
  }

  it("rejects original downloads when downloads are disabled", () => {
    const result = eventSettingsSchema.safeParse({
      ...base,
      allowOriginalDownload: true,
    })
    expect(result.success).toBe(false)
  })

  it("accepts original downloads when downloads are enabled", () => {
    const result = eventSettingsSchema.safeParse({
      ...base,
      allowDownload: true,
      allowOriginalDownload: true,
    })
    expect(result.success).toBe(true)
  })
})

describe("request builders", () => {
  it("omits empty optional fields when creating an event", () => {
    expect(
      toCreateEventRequest({
        name: "Wedding",
        eventDate: "",
        clientName: "",
        clientEmail: "",
      })
    ).toEqual({ name: "Wedding" })
  })

  it("omits an empty password when updating settings", () => {
    expect(
      toUpdateEventSettingsRequest({
        visibility: "password",
        password: "",
        allowDownload: true,
        allowOriginalDownload: false,
        watermarkEnabled: false,
      })
    ).toEqual({
      visibility: "password",
      allowDownload: true,
      allowOriginalDownload: false,
      watermarkEnabled: false,
    })
  })

  it("includes a provided password when updating settings", () => {
    expect(
      toUpdateEventSettingsRequest({
        visibility: "password",
        password: "secret",
        allowDownload: true,
        allowOriginalDownload: false,
        watermarkEnabled: false,
      }).password
    ).toBe("secret")
  })
})
