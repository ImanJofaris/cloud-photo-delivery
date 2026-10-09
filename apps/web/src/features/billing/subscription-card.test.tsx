import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { SubscriptionCard } from "./subscription-card"

type Plan = components["schemas"]["Plan"]
type Subscription = components["schemas"]["Subscription"]
type SubscriptionView = components["schemas"]["SubscriptionView"]

function plan(overrides: Partial<Plan> = {}): Plan {
  return {
    id: "pro",
    name: "Pro",
    priceCents: 9900,
    currency: "MYR",
    interval: "month",
    limits: {
      events: 10,
      photosPerEvent: 20000,
      storageBytes: 107374182400,
      retentionDays: 90,
      apiAccess: true,
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
    planId: "pro",
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

function view(overrides: Partial<SubscriptionView> = {}): SubscriptionView {
  return {
    subscription: subscription(),
    plan: plan(),
    usage: { events: 3, storageBytes: 64 * 1024 ** 3 },
    ...overrides,
  }
}

describe("SubscriptionCard", () => {
  it("shows the plan, current badge, price, renewal, and usage", () => {
    render(
      <SubscriptionCard view={view()} onCancel={vi.fn()} onResume={vi.fn()} />
    )

    expect(screen.getByRole("heading", { name: "Pro" })).toBeInTheDocument()
    expect(screen.getByText("Current plan")).toBeInTheDocument()
    expect(screen.getByText(/99[.,]00/)).toBeInTheDocument()
    expect(screen.getByText(/renews 1 Nov 2026/)).toBeInTheDocument()
    expect(screen.getByText("3 of 10")).toBeInTheDocument()
    expect(screen.getByText("64 GB of 100 GB")).toBeInTheDocument()
  })

  it("links Change plan to the plans section", () => {
    render(
      <SubscriptionCard view={view()} onCancel={vi.fn()} onResume={vi.fn()} />
    )

    expect(screen.getByRole("button", { name: "Change plan" })).toHaveAttribute(
      "href",
      "#plans"
    )
  })

  it("cancels the subscription from the card", async () => {
    const onCancel = vi.fn()
    render(
      <SubscriptionCard view={view()} onCancel={onCancel} onResume={vi.fn()} />
    )

    await userEvent.click(
      screen.getByRole("button", { name: "Cancel subscription" })
    )

    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it("resumes when the subscription ends at the period end", async () => {
    const onResume = vi.fn()
    render(
      <SubscriptionCard
        view={view({ subscription: subscription({ cancelAtPeriodEnd: true }) })}
        onCancel={vi.fn()}
        onResume={onResume}
      />
    )

    expect(screen.getByText(/ends 1 Nov 2026/)).toBeInTheDocument()
    expect(
      screen.queryByRole("button", { name: "Cancel subscription" })
    ).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: "Resume" }))

    expect(onResume).toHaveBeenCalledTimes(1)
  })

  it("shows the Free state without a paid subscription", () => {
    render(
      <SubscriptionCard
        view={view({
          subscription: null,
          plan: plan({ id: "free", name: "Free", priceCents: 0 }),
        })}
        onCancel={vi.fn()}
        onResume={vi.fn()}
      />
    )

    expect(screen.getByRole("heading", { name: "Free" })).toBeInTheDocument()
    expect(
      screen.getByText("No paid subscription. The Free plan applies.")
    ).toBeInTheDocument()
    expect(
      screen.queryByRole("button", { name: "Cancel subscription" })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Change plan" })
    ).toBeInTheDocument()
  })
})
