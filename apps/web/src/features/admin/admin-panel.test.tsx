import { render, screen } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError, type components } from "@workspace/api-client"

const { replace } = vi.hoisted(() => ({ replace: vi.fn() }))

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}))

vi.mock("@/lib/auth/session-provider", () => ({
  useSession: vi.fn(),
}))

vi.mock("./api", () => ({
  useAdminHealth: vi.fn(),
  useAdminStats: vi.fn(),
  useAdminSubscriptions: vi.fn(),
  useAdminUsers: vi.fn(),
}))

import { useSession } from "@/lib/auth/session-provider"

import {
  useAdminHealth,
  useAdminStats,
  useAdminSubscriptions,
  useAdminUsers,
} from "./api"
import { AdminPanel } from "./admin-panel"

type AdminStats = components["schemas"]["AdminStats"]
type AdminUser = components["schemas"]["AdminUser"]
type AdminSubscription = components["schemas"]["AdminSubscription"]
type AdminHealth = components["schemas"]["AdminHealth"]

const stats: AdminStats = {
  users: 12,
  events: 34,
  photos: 5600,
  storageBytes: 1536,
  revenueCents: 4900,
  subscriptions: 3,
}

const health: AdminHealth = {
  status: "degraded",
  queueDepth: 3,
  queue: {
    pending: 2,
    running: 1,
    failed: 4,
    oldestPendingAt: "2026-09-16T11:00:00Z",
  },
  checkedAt: "2026-09-16T12:00:00Z",
}

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

function mockAdmin({
  session = {},
  statsQuery = {},
  healthQuery = {},
  usersQuery = {},
  subscriptionsQuery = {},
}: {
  session?: Record<string, unknown>
  statsQuery?: Record<string, unknown>
  healthQuery?: Record<string, unknown>
  usersQuery?: Record<string, unknown>
  subscriptionsQuery?: Record<string, unknown>
} = {}) {
  vi.mocked(useSession).mockReturnValue({
    status: "authenticated",
    user: {
      id: "u-1",
      email: "admin@example.com",
      businessName: "Admin Co",
      isAdmin: true,
    },
    accessToken: "token",
    login: vi.fn(),
    signup: vi.fn(),
    logout: vi.fn(),
    ...session,
  } as unknown as ReturnType<typeof useSession>)

  vi.mocked(useAdminStats).mockReturnValue({
    data: stats,
    isPending: false,
    isError: false,
    error: null,
    ...statsQuery,
  } as unknown as ReturnType<typeof useAdminStats>)

  vi.mocked(useAdminHealth).mockReturnValue({
    data: health,
    isPending: false,
    isError: false,
    error: null,
    ...healthQuery,
  } as unknown as ReturnType<typeof useAdminHealth>)

  vi.mocked(useAdminUsers).mockReturnValue({
    data: { pages: [{ users: [user()], nextCursor: null }] },
    isPending: false,
    isError: false,
    error: null,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
    ...usersQuery,
  } as unknown as ReturnType<typeof useAdminUsers>)

  vi.mocked(useAdminSubscriptions).mockReturnValue({
    data: { pages: [{ subscriptions: [subscription()], nextCursor: null }] },
    isPending: false,
    isError: false,
    error: null,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
    ...subscriptionsQuery,
  } as unknown as ReturnType<typeof useAdminSubscriptions>)
}

describe("AdminPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAdmin()
  })

  it("renders stats, health, and both tables for an admin", () => {
    render(<AdminPanel />)

    expect(screen.getByRole("heading", { name: "Admin" })).toBeInTheDocument()
    expect(screen.getByText("Read-only overview.")).toBeInTheDocument()
    expect(screen.getAllByText("1.5 KB")).toHaveLength(2)
    expect(screen.getByText(/49[.,]00/)).toBeInTheDocument()
    expect(screen.getByText("5,600")).toBeInTheDocument()
    expect(screen.getByText("degraded")).toBeInTheDocument()
    expect(screen.getByText("Queue depth")).toBeInTheDocument()
    expect(screen.getByText("4")).toBeInTheDocument()
    expect(screen.getAllByText("booth@example.com")).toHaveLength(2)
    expect(screen.getByText("pro")).toBeInTheDocument()
    expect(screen.getByText("active")).toBeInTheDocument()
    expect(useAdminStats).toHaveBeenCalledWith(true)
  })

  it("flattens pages across the two cursor lists", () => {
    mockAdmin({
      usersQuery: {
        data: {
          pages: [
            { users: [user()], nextCursor: "cursor-1" },
            {
              users: [user({ id: "u-2", email: "second@example.com" })],
              nextCursor: null,
            },
          ],
        },
        hasNextPage: false,
      },
      subscriptionsQuery: {
        data: {
          pages: [
            {
              subscriptions: [subscription()],
              nextCursor: "cursor-1",
            },
            {
              subscriptions: [
                subscription({ id: "s-2", planId: "starter" }),
              ],
              nextCursor: null,
            },
          ],
        },
      },
    })

    render(<AdminPanel />)

    expect(screen.getAllByText("booth@example.com").length).toBeGreaterThan(0)
    expect(screen.getByText("second@example.com")).toBeInTheDocument()
    expect(screen.getByText("pro")).toBeInTheDocument()
    expect(screen.getByText("starter")).toBeInTheDocument()
  })

  it("shows skeletons while the session is loading", () => {
    mockAdmin({ session: { status: "loading", user: null } })

    const { container } = render(<AdminPanel />)

    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(
      4
    )
    expect(screen.queryByText("Queue depth")).not.toBeInTheDocument()
    expect(useAdminStats).toHaveBeenCalledWith(false)
  })

  it("shows empty states for both tables", () => {
    mockAdmin({
      usersQuery: { data: { pages: [{ users: [], nextCursor: null }] } },
      subscriptionsQuery: {
        data: { pages: [{ subscriptions: [], nextCursor: null }] },
      },
    })

    render(<AdminPanel />)

    expect(screen.getByText("No operators yet.")).toBeInTheDocument()
    expect(screen.getByText("No subscriptions yet.")).toBeInTheDocument()
  })

  it("maps a mid-session FORBIDDEN without leaking server text", () => {
    mockAdmin({
      statsQuery: {
        data: undefined,
        isError: true,
        error: new ApiError("FORBIDDEN", "raw server detail", 403),
      },
    })

    render(<AdminPanel />)

    expect(screen.getByText("Admin access required.")).toBeInTheDocument()
    expect(screen.queryByText(/raw server detail/)).not.toBeInTheDocument()
    expect(screen.queryByText("5,600")).not.toBeInTheDocument()
  })

  it("redirects authenticated non-admins to the dashboard", () => {
    mockAdmin({
      session: {
        user: { id: "u-2", email: "operator@example.com", isAdmin: false },
      },
    })

    render(<AdminPanel />)

    expect(replace).toHaveBeenCalledWith("/dashboard")
    expect(screen.queryByText("Queue depth")).not.toBeInTheDocument()
    expect(useAdminStats).toHaveBeenCalledWith(false)
  })
})
