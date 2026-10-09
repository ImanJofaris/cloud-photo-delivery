import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError, type components } from "@workspace/api-client"

const { replace, refresh } = vi.hoisted(() => ({
  replace: vi.fn(),
  refresh: vi.fn(),
}))

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace, refresh }),
}))

vi.mock("@/lib/auth/session-provider", () => ({
  useSession: vi.fn(),
}))

vi.mock("./api", () => ({
  useProfile: vi.fn(),
}))

vi.mock("@/features/billing/api", () => ({
  useSubscription: vi.fn(),
}))

import { useSubscription } from "@/features/billing/api"
import { useSession } from "@/lib/auth/session-provider"

import { AccountPanel } from "./account-panel"
import { useProfile } from "./api"

type Profile = components["schemas"]["Profile"]
type Plan = components["schemas"]["Plan"]
type Subscription = components["schemas"]["Subscription"]
type SubscriptionView = components["schemas"]["SubscriptionView"]

const logout = vi.fn()

const profile: Profile = {
  id: "usr_8f3a21c4",
  email: "hello@pixelparty.my",
  businessName: "Pixel Party Photobooth",
  isAdmin: false,
}

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

const subscription: Subscription = {
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
}

const view: SubscriptionView = {
  plan,
  subscription,
  usage: { events: 3, storageBytes: 1_073_741_824 },
}

function mockAccount({
  profileQuery = {},
  subscriptionQuery = {},
}: {
  profileQuery?: Record<string, unknown>
  subscriptionQuery?: Record<string, unknown>
} = {}) {
  vi.mocked(useSession).mockReturnValue({
    status: "authenticated",
    user: {
      id: profile.id,
      email: profile.email,
      businessName: profile.businessName,
      isAdmin: profile.isAdmin,
    },
    accessToken: "token",
    login: vi.fn(),
    signup: vi.fn(),
    logout,
  } as unknown as ReturnType<typeof useSession>)

  vi.mocked(useProfile).mockReturnValue({
    data: profile,
    isPending: false,
    isError: false,
    error: null,
    ...profileQuery,
  } as unknown as ReturnType<typeof useProfile>)

  vi.mocked(useSubscription).mockReturnValue({
    data: view,
    isPending: false,
    isError: false,
    error: null,
    ...subscriptionQuery,
  } as unknown as ReturnType<typeof useSubscription>)
}

describe("AccountPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAccount()
  })

  it("renders the profile details and plan summary", () => {
    render(<AccountPanel />)

    expect(screen.getByRole("heading", { name: "Account" })).toBeInTheDocument()
    expect(screen.getByText("Your operator profile.")).toBeInTheDocument()
    expect(screen.getAllByText("Pixel Party Photobooth").length).toBeGreaterThan(
      0
    )
    expect(screen.getAllByText("hello@pixelparty.my").length).toBeGreaterThan(0)
    expect(screen.getByText("usr_8f3a21c4")).toBeInTheDocument()
    expect(screen.getByText("Operator")).toBeInTheDocument()
    expect(screen.getByText("Pro plan")).toBeInTheDocument()

    expect(screen.getByRole("link", { name: "Manage" })).toHaveAttribute(
      "href",
      "/billing"
    )
    expect(screen.getByText("Storage")).toBeInTheDocument()
    expect(screen.getByText("1.0 GB of 100 GB")).toBeInTheDocument()
    expect(screen.getByText("3 of 10")).toBeInTheDocument()
    expect(
      screen.getByRole("progressbar", { name: "Storage" })
    ).toBeInTheDocument()
    expect(
      screen.getByRole("progressbar", { name: "Events" })
    ).toBeInTheDocument()
    expect(screen.getByText(/renews/)).toBeInTheDocument()
  })

  it("labels admin accounts", () => {
    mockAccount({ profileQuery: { data: { ...profile, isAdmin: true } } })

    render(<AccountPanel />)

    expect(screen.getByText("Admin")).toBeInTheDocument()
    expect(screen.queryByText("Operator")).not.toBeInTheDocument()
  })

  it("shows skeletons while the profile and plan load", () => {
    mockAccount({
      profileQuery: { data: undefined, isPending: true },
      subscriptionQuery: { data: undefined, isPending: true },
    })

    const { container } = render(<AccountPanel />)

    expect(
      container.querySelectorAll('[data-slot="skeleton"]').length
    ).toBeGreaterThan(0)
    expect(screen.queryByText("usr_8f3a21c4")).not.toBeInTheDocument()
    expect(screen.queryByText("Pro plan")).not.toBeInTheDocument()
  })

  it("maps errors without leaking server messages", () => {
    mockAccount({
      profileQuery: {
        data: undefined,
        isError: true,
        error: new ApiError("VALIDATION_ERROR", "raw profile detail", 422),
      },
      subscriptionQuery: {
        data: undefined,
        isError: true,
        error: new ApiError("VALIDATION_ERROR", "raw plan detail", 422),
      },
    })

    render(<AccountPanel />)

    expect(
      screen.getByText("Could not load your profile. Please refresh the page.")
    ).toBeInTheDocument()
    expect(
      screen.getByText("Could not load your plan right now.")
    ).toBeInTheDocument()
    expect(screen.queryByText(/raw (profile|plan) detail/)).not.toBeInTheDocument()
  })

  it("signs out through the session provider", async () => {
    render(<AccountPanel />)

    await userEvent.click(screen.getByRole("button", { name: "Sign out" }))

    await waitFor(() => expect(logout).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/login"))
  })
})
