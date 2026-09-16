import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError, type components } from "@workspace/api-client"

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  },
}))

vi.mock("./api", () => ({
  useSubscribe: vi.fn(),
  useUpgrade: vi.fn(),
  useDowngrade: vi.fn(),
  useCancelSubscription: vi.fn(),
  useResumeSubscription: vi.fn(),
}))

import { toast } from "sonner"

import {
  useCancelSubscription,
  useDowngrade,
  useResumeSubscription,
  useSubscribe,
  useUpgrade,
} from "./api"
import { ChangePlanDialog, openCheckout } from "./change-plan-dialog"

type Plan = components["schemas"]["Plan"]
type SubscribeResult = components["schemas"]["SubscribeResult"]

const plan: Plan = {
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
}

function subscribeResult(url: string): SubscribeResult {
  return {
    subscription: {
      id: "sub-1",
      planId: "starter",
      status: "active",
      provider: "manual",
      providerRef: "ref-1",
      interval: "month",
      currentPeriodStart: "2026-09-13T00:00:00Z",
      currentPeriodEnd: "2026-10-13T00:00:00Z",
      cancelAtPeriodEnd: false,
      createdAt: "2026-09-13T00:00:00Z",
      updatedAt: "2026-09-13T00:00:00Z",
    },
    checkout: {
      providerRef: "ref-1",
      url,
      expiresAt: "2026-09-13T01:00:00Z",
    },
  }
}

describe("openCheckout", () => {
  it("never navigates to a manual:// URL", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null)

    expect(openCheckout("manual://offline")).toBe("offline")
    expect(open).not.toHaveBeenCalled()

    open.mockRestore()
  })

  it("opens http(s) checkout URLs in a new tab", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null)

    expect(openCheckout("https://checkout.example/session")).toBe("opened")
    expect(open).toHaveBeenCalledWith(
      "https://checkout.example/session",
      "_blank",
      "noopener,noreferrer"
    )

    open.mockRestore()
  })
})

describe("ChangePlanDialog", () => {
  const subscribeMutate = vi.fn()
  const upgradeMutate = vi.fn()
  const downgradeMutate = vi.fn()
  const cancelMutate = vi.fn()
  const resumeMutate = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useSubscribe).mockReturnValue({
      mutateAsync: subscribeMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useSubscribe>)
    vi.mocked(useUpgrade).mockReturnValue({
      mutateAsync: upgradeMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useUpgrade>)
    vi.mocked(useDowngrade).mockReturnValue({
      mutateAsync: downgradeMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useDowngrade>)
    vi.mocked(useCancelSubscription).mockReturnValue({
      mutateAsync: cancelMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useCancelSubscription>)
    vi.mocked(useResumeSubscription).mockReturnValue({
      mutateAsync: resumeMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useResumeSubscription>)
  })

  it("shows the offline billing note for manual checkout without navigating", async () => {
    subscribeMutate.mockResolvedValue(
      subscribeResult("manual://offline")
    )
    const open = vi.spyOn(window, "open").mockReturnValue(null)
    const onOpenChange = vi.fn()

    render(
      <ChangePlanDialog
        mode="subscribe"
        plan={plan}
        interval="month"
        open
        onOpenChange={onOpenChange}
      />
    )

    await userEvent.click(screen.getByRole("button", { name: "Subscribe" }))

    expect(await screen.findByText("Offline billing")).toBeInTheDocument()
    expect(open).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)

    open.mockRestore()
  })

  it("opens hosted checkout sessions and closes the dialog", async () => {
    subscribeMutate.mockResolvedValue(
      subscribeResult("https://checkout.example/session")
    )
    const open = vi.spyOn(window, "open").mockReturnValue(null)
    const onOpenChange = vi.fn()

    render(
      <ChangePlanDialog
        mode="subscribe"
        plan={plan}
        interval="month"
        open
        onOpenChange={onOpenChange}
      />
    )

    await userEvent.click(screen.getByRole("button", { name: "Subscribe" }))

    await waitFor(() =>
      expect(open).toHaveBeenCalledWith(
        "https://checkout.example/session",
        "_blank",
        "noopener,noreferrer"
      )
    )
    expect(onOpenChange).toHaveBeenCalledWith(false)

    open.mockRestore()
  })

  it("maps subscription errors without leaking server messages", async () => {
    subscribeMutate.mockRejectedValue(
      new ApiError("SUBSCRIPTION_ALREADY_ACTIVE", "raw server detail", 409)
    )

    render(
      <ChangePlanDialog
        mode="subscribe"
        plan={plan}
        interval="month"
        open
        onOpenChange={vi.fn()}
      />
    )

    await userEvent.click(screen.getByRole("button", { name: "Subscribe" }))

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        expect.stringMatching(/already have an active subscription/i)
      )
    )
    expect(toast.error).not.toHaveBeenCalledWith(
      expect.stringContaining("raw server detail")
    )
  })

  it("confirms cancellation", async () => {
    cancelMutate.mockResolvedValue({})
    const onOpenChange = vi.fn()

    render(
      <ChangePlanDialog
        mode="cancel"
        plan={null}
        interval="month"
        open
        onOpenChange={onOpenChange}
      />
    )

    await userEvent.click(
      screen.getByRole("button", { name: "Cancel subscription" })
    )

    await waitFor(() => expect(cancelMutate).toHaveBeenCalledTimes(1))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
