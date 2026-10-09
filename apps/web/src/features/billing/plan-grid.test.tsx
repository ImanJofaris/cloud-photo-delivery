import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { PlanGrid } from "./plan-grid"

type Plan = components["schemas"]["Plan"]
type Subscription = components["schemas"]["Subscription"]
type SubscriptionView = components["schemas"]["SubscriptionView"]

function plan(overrides: Partial<Plan> = {}): Plan {
  return {
    id: "starter",
    name: "Starter",
    priceCents: 2900,
    currency: "MYR",
    interval: "month",
    limits: {
      events: 5,
      photosPerEvent: 5000,
      storageBytes: 53687091200,
      retentionDays: 30,
      apiAccess: false,
      branding: true,
      originalDownloads: true,
    },
    active: true,
    ...overrides,
  }
}

function subscription(overrides: Partial<Subscription> = {}): Subscription {
  return {
    id: "6c1c9b02-2222-4222-8222-000000000001",
    planId: "starter",
    status: "active",
    provider: "manual",
    providerRef: "ref-1",
    interval: "month",
    currentPeriodStart: "2026-10-01T00:00:00Z",
    currentPeriodEnd: "2026-11-01T00:00:00Z",
    cancelAtPeriodEnd: false,
    createdAt: "2026-09-01T00:00:00Z",
    updatedAt: "2026-09-01T00:00:00Z",
    ...overrides,
  }
}

const free = plan({
  id: "free",
  name: "Free",
  priceCents: 0,
  interval: "month",
})
const starter = plan()
const pro = plan({ id: "pro", name: "Pro", priceCents: 9900 })

function view(overrides: Partial<SubscriptionView> = {}): SubscriptionView {
  return {
    subscription: null,
    plan: free,
    usage: { events: 1, storageBytes: 1024 },
    ...overrides,
  }
}

describe("PlanGrid", () => {
  it("rings the current plan and labels actions from the existing logic", () => {
    render(
      <PlanGrid plans={[free, starter, pro]} view={view()} onAction={vi.fn()} />
    )

    expect(screen.getByRole("heading", { name: "Plans" })).toBeInTheDocument()
    const currentCard = screen.getByText("Free").closest('[data-slot="card"]')
    expect(currentCard).toHaveClass("ring-2")
    expect(currentCard).toHaveClass("ring-primary")
    expect(
      screen.getByRole("button", { name: "Current plan Free" })
    ).toBeDisabled()
    expect(
      screen.getByRole("button", { name: "Subscribe to Starter" })
    ).toBeEnabled()
    expect(
      screen.getByRole("button", { name: "Subscribe to Pro" })
    ).toBeEnabled()
  })

  it("switches plan prices with the Monthly/Yearly control", async () => {
    render(
      <PlanGrid plans={[free, starter, pro]} view={view()} onAction={vi.fn()} />
    )

    expect(screen.getByText(/29[.,]00/)).toBeInTheDocument()
    expect(screen.queryByText(/348[.,]00/)).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: "Yearly" }))

    expect(screen.getByText(/348[.,]00/)).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Yearly" })).toHaveAttribute(
      "aria-pressed",
      "true"
    )
  })

  it("passes the plan, action, and interval to the dialog opener", async () => {
    const onAction = vi.fn()
    render(
      <PlanGrid plans={[free, starter]} view={view()} onAction={onAction} />
    )

    await userEvent.click(
      screen.getByRole("button", { name: "Subscribe to Starter" })
    )

    expect(onAction).toHaveBeenCalledWith(starter, "subscribe", "month")
  })

  it("hides the interval control while a subscription is usable", () => {
    render(
      <PlanGrid
        plans={[free, starter, pro]}
        view={view({
          subscription: subscription(),
          plan: starter,
        })}
        onAction={vi.fn()}
      />
    )

    expect(
      screen.queryByRole("group", { name: "Billing interval" })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Upgrade to Pro" })
    ).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Downgrade to Free" })
    ).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Current plan Starter" })
    ).toBeDisabled()
  })
})
