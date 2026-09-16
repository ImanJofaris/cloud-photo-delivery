import { describe, expect, it } from "vitest"

import { isNavActive, visibleNavItems } from "./app-sidebar"

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

describe("visibleNavItems", () => {
  it("includes Admin right after Billing for admins", () => {
    const titles = visibleNavItems(true).map((item) => item.title)
    expect(titles).toEqual([
      "Dashboard",
      "Analytics",
      "Events",
      "Devices",
      "Branding",
      "Billing",
      "Admin",
      "Account",
    ])
  })

  it("excludes Admin for ordinary operators", () => {
    const titles = visibleNavItems(false).map((item) => item.title)
    expect(titles).not.toContain("Admin")
    expect(titles).toEqual([
      "Dashboard",
      "Analytics",
      "Events",
      "Devices",
      "Branding",
      "Billing",
      "Account",
    ])
  })

  it("preserves the declared order in both cases", () => {
    const adminHrefs = visibleNavItems(true).map((item) => item.href)
    expect(adminHrefs.indexOf("/admin")).toBe(adminHrefs.indexOf("/billing") + 1)
    expect(adminHrefs.indexOf("/account")).toBe(adminHrefs.indexOf("/admin") + 1)
  })
})
