import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { AdminUsersTable } from "./admin-users-table"

type AdminUser = components["schemas"]["AdminUser"]

function user(overrides: Partial<AdminUser> = {}): AdminUser {
  return {
    id: "u-1",
    email: "booth@example.com",
    businessName: "Booth Co",
    isAdmin: false,
    storageBytes: 1536,
    eventCount: 12,
    createdAt: "2026-09-01T00:00:00Z",
    ...overrides,
  }
}

describe("AdminUsersTable", () => {
  it("renders operators with roles, counts, and storage", () => {
    render(
      <AdminUsersTable
        users={[
          user(),
          user({
            id: "u-2",
            email: "admin@example.com",
            businessName: "Admin Co",
            isAdmin: true,
            eventCount: 3,
            storageBytes: 0,
          }),
        ]}
        hasNextPage={false}
        isFetchingNextPage={false}
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getByText("booth@example.com")).toBeInTheDocument()
    expect(screen.getByText("Admin Co")).toBeInTheDocument()
    expect(screen.getByText("Admin")).toBeInTheDocument()
    expect(screen.getByText("Operator")).toBeInTheDocument()
    expect(screen.getByText("12")).toBeInTheDocument()
    expect(screen.getByText("1.5 KB")).toBeInTheDocument()
    expect(screen.getByText("0 B")).toBeInTheDocument()
    expect(screen.getAllByText(/Sept 2026/)).toHaveLength(2)
    expect(
      screen.queryByRole("button", { name: "Load more" })
    ).not.toBeInTheDocument()
  })

  it("shows an empty state", () => {
    render(
      <AdminUsersTable
        users={[]}
        hasNextPage={false}
        isFetchingNextPage={false}
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getByText("No operators yet.")).toBeInTheDocument()
  })

  it("loads more pages", async () => {
    const onLoadMore = vi.fn()
    render(
      <AdminUsersTable
        users={[user()]}
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
      <AdminUsersTable
        users={[user()]}
        hasNextPage
        isFetchingNextPage
        onLoadMore={vi.fn()}
      />
    )

    expect(screen.getByRole("button", { name: "Loading..." })).toBeDisabled()
  })
})
