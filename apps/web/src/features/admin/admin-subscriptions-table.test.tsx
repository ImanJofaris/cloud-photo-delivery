import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { AdminSubscriptionsTable } from "./admin-subscriptions-table"

type AdminSubscription = components["schemas"]["AdminSubscription"]

function subscription(
  overrides: Partial<AdminSubscription> = {}
): AdminSubscription {
  return {
    id: "s-1",
    userId: "u-1",
    userEmail: "booth@example.com",
    planId: "pro",
    status: "active",
    interval: "month",
    currentPeriodEnd: "2026-10-01T00:00:00Z",
    cancelAtPeriodEnd: false,
    createdAt: "2026-09-01T00:00:00Z",
    ...overrides,
  }
}

describe("AdminSubscriptionsTable", () => {
  it("renders operator, plan, status, interval, period end, and cancel flag", () => {
    render(
      <AdminSubscriptionsTable
        subscriptions={[
          subscription(),
          subscription({
            id: "s-2",
            userEmail: "late@example.com",
            planId: "starter",
            status: "past_due",
            interval: "year",
            currentPeriodEnd: null,
            cancelAtPeriodEnd: true,
          }),
        ]}
        hasNextPage={false}
        isFetchingNextPage={false}
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getByText("booth@example.com")).toBeInTheDocument()
    expect(screen.getByText("pro")).toBeInTheDocument()
    expect(screen.getByText("active")).toBeInTheDocument()
    expect(screen.getByText("month")).toBeInTheDocument()
    expect(screen.getByText("Yes")).toBeInTheDocument()
    expect(screen.getByText("No")).toBeInTheDocument()
    expect(screen.getByText("starter")).toBeInTheDocument()
    expect(screen.getByText("past_due")).toBeInTheDocument()
    expect(screen.getByText("year")).toBeInTheDocument()
    expect(screen.getAllByText("—")).toHaveLength(1)
    expect(
      screen.queryByRole("button", { name: "Load more" })
    ).not.toBeInTheDocument()
  })

  it("shows an empty state", () => {
    render(
      <AdminSubscriptionsTable
        subscriptions={[]}
        hasNextPage={false}
        isFetchingNextPage={false}
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getByText("No subscriptions yet.")).toBeInTheDocument()
  })

  it("loads more pages", async () => {
    const onLoadMore = vi.fn()
    render(
      <AdminSubscriptionsTable
        subscriptions={[subscription()]}
        hasNextPage
        isFetchingNextPage={false}
        onLoadMore={onLoadMore}
      />
    )

    await userEvent.click(screen.getByRole("button", { name: "Load more" }))

    expect(onLoadMore).toHaveBeenCalledTimes(1)
  })

  it("disables load more while fetching", () => {
    render(
      <AdminSubscriptionsTable
        subscriptions={[subscription()]}
        hasNextPage
        isFetchingNextPage
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getByRole("button", { name: "Loading..." })).toBeDisabled()
  })
})
