import { describe, expect, it } from "vitest"

import { isNavActive } from "./app-sidebar"

describe("isNavActive", () => {
  it("matches the exact route", () => {
    expect(isNavActive("/events", "/events")).toBe(true)
    expect(isNavActive("/billing", "/billing")).toBe(true)
  })

  it("matches nested routes", () => {
    expect(isNavActive("/events/123", "/events")).toBe(true)
    expect(isNavActive("/events/123/settings", "/events")).toBe(true)
  })

  it("does not match lookalike prefixes", () => {
    expect(isNavActive("/events-archive", "/events")).toBe(false)
    expect(isNavActive("/billing-history", "/billing")).toBe(false)
    expect(isNavActive("/dashboard/settings", "/events")).toBe(false)
  })
})
