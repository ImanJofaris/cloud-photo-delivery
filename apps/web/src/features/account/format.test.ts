import { describe, expect, it } from "vitest"

import type { components } from "@workspace/api-client"

import { formatMYR, planPriceCents } from "@/features/billing/format"

import { initials, planSummary, roleLabel } from "./format"

type Plan = components["schemas"]["Plan"]
type Subscription = components["schemas"]["Subscription"]
type SubscriptionView = components["schemas"]["SubscriptionView"]

const plan: Plan = {
  id: "pro",
  name: "Pro",
  priceCents: 9900,
  currency: "MYR",
  interval: "month",
  limits: {
    events: 10,
    photosPerEvent: 500,
    storageBytes: 107_374_182_400,
    retentionDays: 90,
    apiAccess: true,
    branding: true,
    originalDownloads: true,
  },
  active: true,
}

function subscription(overrides: Partial<Subscription> = {}): Subscription {
  return {
    id: "11111111-1111-1111-1111-111111111111",
    planId: "pro",
    status: "active",
    provider: null,
    providerRef: null,
    interval: "month",
    currentPeriodStart: "2026-10-01T00:00:00Z",
    currentPeriodEnd: "2026-11-01T00:00:00Z",
    cancelAtPeriodEnd: false,
    createdAt: "2026-09-01T00:00:00Z",
    updatedAt: "2026-09-01T00:00:00Z",
    ...overrides,
  }
}

function view(overrides: Partial<SubscriptionView> = {}): SubscriptionView {
  return {
    plan,
    subscription: subscription(),
    usage: { events: 3, storageBytes: 1_073_741_824 },
    ...overrides,
  }
}

describe("initials", () => {
  it("takes the first letter of the first two words", () => {
    expect(initials("Pixel Party Photobooth")).toBe("PP")
  })

  it("falls back to a single letter or an empty string", () => {
    expect(initials("Pixel")).toBe("P")
    expect(initials("")).toBe("")
    expect(initials(null)).toBe("")
    expect(initials(undefined)).toBe("")
  })
})

describe("roleLabel", () => {
  it("labels admins and operators", () => {
    expect(roleLabel({ isAdmin: true })).toBe("Admin")
    expect(roleLabel({ isAdmin: false })).toBe("Operator")
    expect(roleLabel({})).toBe("Operator")
  })
})

describe("planSummary", () => {
  it("shows the plan, price, interval, and renewal", () => {
    const summary = planSummary(view())
    expect(summary).toContain(`Pro · ${formatMYR(9900)} / month`)
    expect(summary).toMatch(/renews/)
  })

  it("says cancels when the subscription ends at the period end", () => {
    expect(
      planSummary(
        view({ subscription: subscription({ cancelAtPeriodEnd: true }) })
      )
    ).toMatch(/cancels/)
  })

  it("charges twelve months for a yearly subscription", () => {
    const summary = planSummary(
      view({ subscription: subscription({ interval: "year" }) })
    )
    expect(summary).toContain(
      `${formatMYR(planPriceCents(plan, "year"))} / year`
    )
  })

  it("describes the free plan when there is no paid subscription", () => {
    expect(planSummary(view({ subscription: null }))).toBe(
      "Pro plan · no paid subscription"
    )
    expect(
      planSummary(view({ subscription: subscription({ status: "canceled" }) }))
    ).toBe("Pro plan · no paid subscription")
  })
})
