import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { InvoicesTable } from "./invoices-table"

type Invoice = components["schemas"]["Invoice"]

function invoice(overrides: Partial<Invoice> = {}): Invoice {
  return {
    id: "3f2a91c4-1111-4111-8111-000000000001",
    subscriptionId: "6c1c9b02-2222-4222-8222-000000000001",
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
  it("renders refs, amounts, dates, and status badges", () => {
    render(
      <InvoicesTable
        invoices={[
          invoice(),
          invoice({
            id: "8b7d4e02-2222-4222-8222-000000000002",
            status: "paid",
            paidAt: "2026-09-02T00:00:00Z",
          }),
          invoice({
            id: "c4a6f901-3333-4333-8333-000000000003",
            status: "void",
          }),
        ]}
        hasNextPage={false}
        isFetchingNextPage={false}
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getAllByText(/29[.,]00/)).toHaveLength(3)
    expect(screen.getByText("INV-3F2A91C4")).toBeInTheDocument()
    expect(screen.getByText("INV-8B7D4E02")).toBeInTheDocument()
    expect(screen.getByText("INV-C4A6F901")).toBeInTheDocument()
    expect(screen.getByText("Open")).toBeInTheDocument()
    expect(screen.getByText("Paid")).toBeInTheDocument()
    expect(screen.getByText("Void")).toBeInTheDocument()
    expect(screen.getAllByRole("cell")).toHaveLength(12)
  })

  it("shows the full invoice id on the reference cell", () => {
    const id = "3f2a91c4-1111-4111-8111-000000000001"
    render(
      <InvoicesTable
        invoices={[invoice({ id })]}
        hasNextPage={false}
        isFetchingNextPage={false}
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getByTitle(id)).toBeInTheDocument()
  })

  it("does not render a receipt download without an API url", () => {
    render(
      <InvoicesTable
        invoices={[invoice()]}
        hasNextPage={false}
        isFetchingNextPage={false}
        onLoadMore={vi.fn()}
      />
    )

    expect(
      screen.queryByRole("columnheader", { name: /receipt/i })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole("link", { name: /receipt|download/i })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole("button", { name: /receipt|download/i })
    ).not.toBeInTheDocument()
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

    await userEvent.click(
      screen.getByRole("button", { name: "Load older invoices" })
    )

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
