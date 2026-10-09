import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

const { pushMock, toastError, toastSuccess } = vi.hoisted(() => ({
  pushMock: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}))

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}))

vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: toastError },
}))

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>()
  return {
    ...actual,
    useCreateEvent: vi.fn(),
    applyEventSettings: vi.fn(),
  }
})

vi.mock("@/features/billing/api", () => ({
  useSubscription: vi.fn(),
}))

vi.mock("@/features/branding/api", () => ({
  useBranding: vi.fn(),
}))

import { useBranding } from "@/features/branding/api"
import { useSubscription } from "@/features/billing/api"

import { applyEventSettings, useCreateEvent } from "./api"
import { EventForm } from "./event-form"

type EventWithSettings = components["schemas"]["EventWithSettings"]
type SubscriptionView = components["schemas"]["SubscriptionView"]
type Branding = components["schemas"]["Branding"]

const createdEvent: EventWithSettings = {
  event: {
    id: "event-1",
    name: "Sarah's Birthday",
    slug: "sarahs-birthday",
    eventDate: "2026-10-18",
    status: "upcoming",
    storageBytes: 0,
    photoCount: 0,
    guestCount: 0,
    expiresAt: "2026-11-17T00:00:00Z",
    createdAt: "2026-10-09T00:00:00Z",
    updatedAt: "2026-10-09T00:00:00Z",
  },
  settings: {
    eventId: "event-1",
    visibility: "public",
    passwordProtected: false,
    allowDownload: true,
    allowOriginalDownload: false,
    watermarkEnabled: false,
    updatedAt: "2026-10-09T00:00:00Z",
  },
}

function subscriptionView(brandingAllowed: boolean): SubscriptionView {
  return {
    subscription: null,
    plan: {
      id: "free",
      name: "Free",
      priceCents: 0,
      currency: "MYR",
      interval: "month",
      limits: {
        events: 10,
        photosPerEvent: 100,
        storageBytes: 10 * 1024 * 1024 * 1024,
        retentionDays: 30,
        apiAccess: false,
        branding: brandingAllowed,
        originalDownloads: false,
      },
      active: true,
    },
    usage: { events: 3, storageBytes: 1024 * 1024 * 1024 },
  }
}

const branding: Branding = {
  businessName: "Aina Booth",
  logoUrl: null,
  profileImageUrl: null,
  primaryColor: "#ff0066",
  secondaryColor: null,
  contactEmail: null,
  contactPhone: null,
  websiteUrl: null,
  updatedAt: null,
}

describe("EventForm", () => {
  const createMock = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    createMock.mockResolvedValue(createdEvent)
    vi.mocked(useCreateEvent).mockReturnValue({
      mutateAsync: createMock,
      isPending: false,
    } as unknown as ReturnType<typeof useCreateEvent>)
    vi.mocked(applyEventSettings).mockResolvedValue(createdEvent.settings)
    vi.mocked(useSubscription).mockReturnValue({
      data: subscriptionView(false),
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useSubscription>)
    vi.mocked(useBranding).mockReturnValue({
      data: branding,
    } as unknown as ReturnType<typeof useBranding>)
  })

  it("applies the gallery defaults after creation and opens the event", async () => {
    render(<EventForm />)

    fireEvent.change(screen.getByLabelText("Event name"), {
      target: { value: "Sarah's Birthday" },
    })
    await userEvent.click(screen.getByRole("button", { name: "Create event" }))

    await waitFor(() =>
      expect(createMock).toHaveBeenCalledWith({ name: "Sarah's Birthday" })
    )
    await waitFor(() =>
      expect(applyEventSettings).toHaveBeenCalledWith("event-1", {
        visibility: "public",
        allowDownload: true,
        allowOriginalDownload: false,
        watermarkEnabled: false,
      })
    )
    expect(pushMock).toHaveBeenCalledWith("/events/event-1")
    expect(toastSuccess).toHaveBeenCalledWith("Event created")
  })

  it("still opens the event when applying the defaults fails", async () => {
    vi.mocked(applyEventSettings).mockRejectedValue(new Error("boom"))
    render(<EventForm />)

    fireEvent.change(screen.getByLabelText("Event name"), {
      target: { value: "Sarah's Birthday" },
    })
    await userEvent.click(screen.getByRole("button", { name: "Create event" }))

    await waitFor(() =>
      expect(pushMock).toHaveBeenCalledWith("/events/event-1")
    )
    expect(toastError).toHaveBeenCalledWith(
      "Event created, but the gallery defaults could not be saved. You can change them in Settings."
    )
  })

  it("previews the event from the form values", async () => {
    render(<EventForm />)

    fireEvent.change(screen.getByLabelText("Event name"), {
      target: { value: "Sarah's Birthday" },
    })
    fireEvent.change(screen.getByLabelText("Location"), {
      target: { value: "Petaling Jaya" },
    })

    expect(screen.getByText("Sarah's Birthday")).toBeInTheDocument()
    expect(screen.getByText(/Petaling Jaya/)).toBeInTheDocument()
    expect(screen.getByText("Anyone with the link")).toBeInTheDocument()
    expect(screen.getByText("Downloads on")).toBeInTheDocument()
  })

  it("requires a password for password-protected galleries", async () => {
    render(<EventForm />)

    fireEvent.change(screen.getByLabelText("Event name"), {
      target: { value: "Sarah's Birthday" },
    })
    await userEvent.click(screen.getByLabelText("Visibility"))
    await userEvent.click(
      await screen.findByRole("option", { name: "Password protected" })
    )
    await userEvent.click(screen.getByRole("button", { name: "Create event" }))

    expect(createMock).not.toHaveBeenCalled()
    expect(
      await screen.findByText("Enter a gallery password")
    ).toBeInTheDocument()
  })

  it("shows usage from the subscription", () => {
    render(<EventForm />)

    expect(screen.getByText("3 of 10")).toBeInTheDocument()
    expect(screen.getByText("1.0 GB of 10 GB")).toBeInTheDocument()
  })

  it("marks custom branding as plan-gated", () => {
    render(<EventForm />)

    expect(
      screen.getByText(/Custom branding and colors are available on paid plans/)
    ).toBeInTheDocument()
  })

  it("shows the plan branding when the plan includes it", () => {
    vi.mocked(useSubscription).mockReturnValue({
      data: subscriptionView(true),
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useSubscription>)

    render(<EventForm />)

    expect(screen.getByText("by Aina Booth")).toBeInTheDocument()
    expect(
      screen.queryByText(/Custom branding and colors are available on paid plans/)
    ).not.toBeInTheDocument()
  })
})
