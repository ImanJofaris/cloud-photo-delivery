import { describe, expect, it } from "vitest"

import { ApiError, type components } from "@workspace/api-client"

import { billingErrorMessage } from "./errors"
import {
  formatByteLimit,
  formatLimit,
  formatMYR,
  isHttpUrl,
  isUsableSubscription,
  planAction,
  planActionLabel,
  planPriceCents,
  usagePercent,
} from "./format"

type Plan = components["schemas"]["Plan"]
type Subscription = components["schemas"]["Subscription"]

function plan(overrides: Partial<Plan> = {}): Plan {
  return {
    id: "starter",
    name: "Starter",
    priceCents: 2900,
    currency: "MYR",
    interval: "month",
    limits: {
      activeEvents: 5,
      photosPerEvent: 5000,
      storageBytes: 53687091200,
      retentionDays: 30,
      apiAccess: false,
    },
    active: true,
    ...overrides,
  }
}

function subscription(
  overrides: Partial<Subscription> = {}
): Subscription {
  return {
    id: "sub-1",
    planId: "starter",
    status: "active",
    provider: "manual",
    providerRef: "ref-1",
    interval: "month",
    currentPeriodStart: "2026-09-01T00:00:00Z",
    currentPeriodEnd: "2026-10-01T00:00:00Z",
    cancelAtPeriodEnd: false,
    createdAt: "2026-09-01T00:00:00Z",
    updatedAt: "2026-09-01T00:00:00Z",
    ...overrides,
  }
}

describe("usage formatting", () => {
  it("renders zero limits as Unlimited", () => {
    expect(formatLimit(0)).toBe("Unlimited")
    expect(formatByteLimit(0)).toBe("Unlimited")
  })

  it("renders numeric limits", () => {
    expect(formatLimit(5)).toBe("5")
    expect(formatByteLimit(53687091200)).toContain("GB")
  })

  it("formats MYR amounts", () => {
    expect(formatMYR(2900)).toMatch(/29[.,]00/)
  })

  it("caps usage percent at 100", () => {
    expect(usagePercent(3, 5)).toBe(60)
    expect(usagePercent(12, 5)).toBe(100)
    expect(usagePercent(3, 0)).toBe(0)
  })
})

describe("planAction", () => {
  it("marks Free as current when there is no usable subscription", () => {
    expect(
      planAction({
        plan: plan({ id: "free", name: "Free", priceCents: 0 }),
        effectivePlanId: "free",
        effectivePriceCents: 0,
        hasUsableSubscription: false,
      })
    ).toBe("current")
  })

  it("offers Subscribe for paid plans without a usable subscription", () => {
    expect(
      planAction({
        plan: plan(),
        effectivePlanId: "free",
        effectivePriceCents: 0,
        hasUsableSubscription: false,
      })
    ).toBe("subscribe")
  })

  it("selects upgrade and downgrade by price", () => {
    expect(
      planAction({
        plan: plan({ id: "pro", priceCents: 5900 }),
        effectivePlanId: "starter",
        effectivePriceCents: 2900,
        hasUsableSubscription: true,
      })
    ).toBe("upgrade")
    expect(
      planAction({
        plan: plan({ id: "free", priceCents: 0 }),
        effectivePlanId: "starter",
        effectivePriceCents: 2900,
        hasUsableSubscription: true,
      })
    ).toBe("downgrade")
    expect(
      planAction({
        plan: plan(),
        effectivePlanId: "starter",
        effectivePriceCents: 2900,
        hasUsableSubscription: true,
      })
    ).toBe("current")
  })

  it("labels every action", () => {
    expect(planActionLabel("subscribe")).toBe("Subscribe")
    expect(planActionLabel("upgrade")).toBe("Upgrade")
    expect(planActionLabel("downgrade")).toBe("Downgrade")
    expect(planActionLabel("current")).toBe("Current plan")
  })

  it("prices yearly billing at twelve months", () => {
    expect(planPriceCents(plan(), "year")).toBe(2900 * 12)
  })
})

describe("subscription helpers", () => {
  it("treats terminal statuses as unusable", () => {
    expect(isUsableSubscription(subscription())).toBe(true)
    expect(
      isUsableSubscription(subscription({ status: "past_due" }))
    ).toBe(true)
    expect(
      isUsableSubscription(subscription({ status: "expired" }))
    ).toBe(false)
    expect(
      isUsableSubscription(subscription({ status: "canceled" }))
    ).toBe(false)
    expect(isUsableSubscription(null)).toBe(false)
  })

  it("recognises http(s) checkout URLs", () => {
    expect(isHttpUrl("https://checkout.example/session")).toBe(true)
    expect(isHttpUrl("http://checkout.example")).toBe(true)
    expect(isHttpUrl("manual://offline")).toBe(false)
  })
})

describe("billingErrorMessage", () => {
  it("maps known codes to copy", () => {
    expect(
      billingErrorMessage(
        new ApiError("SUBSCRIPTION_ALREADY_ACTIVE", "server message", 409)
      )
    ).toMatch(/already have an active subscription/i)
    expect(
      billingErrorMessage(
        new ApiError("INVALID_STATUS_TRANSITION", "server message", 409)
      )
    ).toMatch(/not available right now/i)
    expect(
      billingErrorMessage(new ApiError("PLAN_LIMIT_REACHED", "server", 402))
    ).toMatch(/plan's limit/i)
  })

  it("never leaks the raw server message", () => {
    const message = billingErrorMessage(
      new ApiError("SECRET_CODE", "raw server detail", 500)
    )
    expect(message).not.toContain("raw server detail")
  })
})
