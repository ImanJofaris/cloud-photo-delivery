import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

vi.mock("./api", () => ({
  useEvents: vi.fn(),
}))

vi.mock("@/features/photos/api", () => ({
  usePhotoThumbnail: vi.fn(),
}))

import { usePhotoThumbnail } from "@/features/photos/api"

import { useEvents } from "./api"
import { EventsList, STATUS_OPTIONS } from "./events-list"

type Event = components["schemas"]["Event"]

const DAY_MS = 86_400_000

function event(overrides: Partial<Event> = {}): Event {
  return {
    id: "event-1",
    name: "Wedding",
    slug: "wedding",
    eventDate: "2026-09-01",
    status: "active",
    storageBytes: 0,
    photoCount: 4,
    guestCount: 0,
    expiresAt: null,
    createdAt: "2026-09-01T00:00:00Z",
    updatedAt: "2026-09-01T00:00:00Z",
    ...overrides,
  }
}

function mockEvents(events: Event[]) {
  vi.mocked(useEvents).mockReturnValue({
    data: { pages: [{ items: events, nextCursor: null }] },
    isPending: false,
    isError: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useEvents>)
}

class NoopIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

describe("EventsList", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal("IntersectionObserver", NoopIntersectionObserver)
    vi.mocked(usePhotoThumbnail).mockReturnValue({
      data: undefined,
      isPending: false,
    } as unknown as ReturnType<typeof usePhotoThumbnail>)
    mockEvents([])
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("offers the expired status filter", () => {
    expect(STATUS_OPTIONS).toContain("expired")
  })

  it("filters by expired when the chip is pressed", async () => {
    render(<EventsList />)

    await userEvent.click(screen.getByRole("button", { name: "Expired" }))

    expect(useEvents).toHaveBeenLastCalledWith({ q: "", status: "expired" })
    expect(screen.getByRole("button", { name: "Expired" })).toHaveAttribute(
      "aria-pressed",
      "true"
    )
  })

  it("searches by name", async () => {
    render(<EventsList />)

    await userEvent.type(
      screen.getByPlaceholderText("Search events"),
      "wedding"
    )

    await waitFor(() =>
      expect(useEvents).toHaveBeenLastCalledWith({ q: "wedding", status: "all" })
    )
  })

  it("renders client, location, and photo count in a row", () => {
    mockEvents([
      event({
        id: "event-row",
        name: "Aina & Haziq Wedding",
        clientName: "Aina",
        location: "Dewan Perdana Felda, KL",
        photoCount: 12,
        coverPhotoId: "photo-1",
      }),
    ])

    render(<EventsList />)

    expect(screen.getByText("Aina & Haziq Wedding")).toBeInTheDocument()
    expect(screen.getByText("Aina")).toBeInTheDocument()
    expect(screen.getByText("Dewan Perdana Felda, KL")).toBeInTheDocument()
    expect(screen.getByText("12 photos")).toBeInTheDocument()
    expect(usePhotoThumbnail).toHaveBeenCalledWith("photo-1", false)
  })

  it("hints only when an event expires within seven days", () => {
    mockEvents([
      event({
        id: "event-soon",
        name: "Soon",
        expiresAt: new Date(Date.now() + 3 * DAY_MS - 3_600_000).toISOString(),
      }),
      event({
        id: "event-later",
        name: "Later",
        expiresAt: new Date(Date.now() + 30 * DAY_MS).toISOString(),
      }),
    ])

    render(<EventsList />)

    expect(screen.getByText("Expires in 3 days")).toBeInTheDocument()
    expect(screen.queryByText(/Expires in 30 days/)).not.toBeInTheDocument()
  })

  it("shows an expired hint on expired events", () => {
    mockEvents([
      event({
        id: "event-expired",
        name: "Old",
        status: "expired",
        expiresAt: new Date(Date.now() - 2 * DAY_MS + 3_600_000).toISOString(),
      }),
    ])

    render(<EventsList />)

    expect(screen.getByText("Expired 2 days ago")).toBeInTheDocument()
  })
})
