import { describe, expect, it } from "vitest"

import type { components } from "@workspace/api-client"

import {
  brandingValuesFrom,
  brandingSchema,
  MAX_BRANDING_ASSET_BYTES,
  toBrandingPatch,
  validateBrandingAsset,
} from "./schema"

type Branding = components["schemas"]["Branding"]

const baseValues = {
  businessName: "",
  primaryColor: "",
  secondaryColor: "",
  contactEmail: "",
  contactPhone: "",
  websiteUrl: "",
}

describe("brandingSchema", () => {
  it("accepts empty optional values", () => {
    expect(brandingSchema.safeParse(baseValues).success).toBe(true)
  })

  it("rejects alpha or malformed colors", () => {
    expect(
      brandingSchema.safeParse({ ...baseValues, primaryColor: "#11223344" })
        .success
    ).toBe(false)
    expect(
      brandingSchema.safeParse({ ...baseValues, primaryColor: "blue" }).success
    ).toBe(false)
  })

  it("normalizes colors to lowercase", () => {
    const parsed = brandingSchema.parse({
      ...baseValues,
      primaryColor: "#AABBCC",
    })
    expect(parsed.primaryColor).toBe("#aabbcc")
  })

  it("rejects invalid email and website values", () => {
    expect(
      brandingSchema.safeParse({ ...baseValues, contactEmail: "not-an-email" })
        .success
    ).toBe(false)
    expect(
      brandingSchema.safeParse({
        ...baseValues,
        websiteUrl: "ftp://booth.example",
      }).success
    ).toBe(false)
  })
})

describe("validateBrandingAsset", () => {
  it("accepts supported types under 5 MB", () => {
    expect(
      validateBrandingAsset({
        name: "logo.png",
        type: "image/png",
        size: 1024,
      })
    ).toBeNull()
  })

  it("rejects unsupported types", () => {
    expect(
      validateBrandingAsset({
        name: "logo.gif",
        type: "image/gif",
        size: 1024,
      })
    ).toMatch(/PNG, JPEG, or WebP/)
  })

  it("rejects assets over 5 MB", () => {
    expect(
      validateBrandingAsset({
        name: "logo.png",
        type: "image/png",
        size: MAX_BRANDING_ASSET_BYTES + 1,
      })
    ).toMatch(/5 MB/)
  })
})

describe("toBrandingPatch", () => {
  const initial: Branding = {
    businessName: "Booth Co",
    primaryColor: "#112233",
    secondaryColor: null,
    contactEmail: "hello@booth.example",
    contactPhone: "+60123",
    websiteUrl: null,
    logoUrl: "https://r2.test/logo.png",
    profileImageUrl: null,
    updatedAt: "2026-09-13T10:00:00Z",
  }

  it("omits unchanged fields", () => {
    const values = brandingValuesFrom(initial)
    expect(
      toBrandingPatch({
        initial,
        values,
        logoKey: null,
        profileImageKey: null,
      })
    ).toEqual({})
  })

  it("includes only the changed field and normalizes colors", () => {
    const values = { ...brandingValuesFrom(initial), primaryColor: "#AABBCC" }
    expect(
      toBrandingPatch({ initial, values, logoKey: null, profileImageKey: null })
    ).toEqual({ primaryColor: "#aabbcc" })
  })

  it("sends an empty string to clear a field", () => {
    const values = { ...brandingValuesFrom(initial), contactPhone: "" }
    expect(
      toBrandingPatch({ initial, values, logoKey: null, profileImageKey: null })
    ).toEqual({ contactPhone: "" })
  })

  it("includes asset keys only when they changed", () => {
    const values = brandingValuesFrom(initial)
    expect(
      toBrandingPatch({
        initial,
        values,
        logoKey: "tenant/u/branding/logo/new.png",
        profileImageKey: "",
      })
    ).toEqual({
      logoKey: "tenant/u/branding/logo/new.png",
      profileImageKey: "",
    })
  })
})
