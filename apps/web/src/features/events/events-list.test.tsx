import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

vi.mock("./api", () => ({
  useEvents: vi.fn(),
}))

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
  } as unknown as ReturnType<typeof useEvents>)
}

describe("EventsList", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockEvents([])
  })

  it("offers the expired status filter", () => {
    expect(STATUS_OPTIONS).toContain("expired")
  })

  it("filters by expired", async () => {
    render(<EventsList />)

    await userEvent.click(screen.getByRole("combobox"))
    await userEvent.click(
      await screen.findByRole("option", { name: "Expired" })
    )

    expect(useEvents).toHaveBeenLastCalledWith({ q: "", status: "expired" })
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
