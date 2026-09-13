import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { InvoicesTable } from "./invoices-table"

type Invoice = components["schemas"]["Invoice"]

function invoice(overrides: Partial<Invoice> = {}): Invoice {
  return {
    id: "inv-1",
    subscriptionId: "sub-1",
    amountCents: 2900,
    currency: "MYR",
    status: "open",
    providerRef: "inv_1",
    issuedAt: "2026-09-01T00:00:00Z",
    paidAt: null,
    ...overrides,
  }
}

describe("InvoicesTable", () => {
  it("renders amounts, statuses, and dates", () => {
    render(
      <InvoicesTable
        invoices={[
          invoice(),
          invoice({ id: "inv-2", status: "paid", paidAt: "2026-09-02T00:00:00Z" }),
          invoice({ id: "inv-3", status: "void" }),
        ]}
        hasNextPage={false}
        isFetchingNextPage={false}
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getAllByText(/29[.,]00/)).toHaveLength(3)
    expect(screen.getByText("open")).toBeInTheDocument()
    expect(screen.getByText("paid")).toBeInTheDocument()
    expect(screen.getByText("void")).toBeInTheDocument()
  })

  it("shows an empty state", () => {
    render(
      <InvoicesTable
        invoices={[]}
        hasNextPage={false}
        isFetchingNextPage={false}
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getByText("No invoices yet.")).toBeInTheDocument()
  })

  it("loads more pages", async () => {
    const onLoadMore = vi.fn()
    render(
      <InvoicesTable
        invoices={[invoice()]}
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
      <InvoicesTable
        invoices={[invoice()]}
        hasNextPage
        isFetchingNextPage
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getByRole("button", { name: "Loading..." })).toBeDisabled()
  })
})
