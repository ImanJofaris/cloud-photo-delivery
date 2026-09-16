import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError, type components } from "@workspace/api-client"
import { Button } from "@workspace/ui/components/button"

vi.mock("./api", () => ({
  useExtendEvent: vi.fn(),
}))

import { useExtendEvent } from "./api"
import { ExtendExpiryDialog } from "./extend-expiry-dialog"

type Event = components["schemas"]["Event"]

function eventFixture(overrides: Partial<Event> = {}): Event {
  return {
    id: "event-1",
    name: "Wedding",
    slug: "wedding",
    eventDate: "2026-09-01",
    status: "active",
    storageBytes: 0,
    photoCount: 0,
    guestCount: 0,
    expiresAt: "2026-12-01T00:00:00Z",
    createdAt: "2026-09-01T00:00:00Z",
    updatedAt: "2026-09-01T00:00:00Z",
    ...overrides,
  }
}

function renderDialog(status: Event["status"]) {
  return render(
    <ExtendExpiryDialog
      eventId="event-1"
      status={status}
      trigger={<Button>Extend</Button>}
    />
  )
}

describe("ExtendExpiryDialog", () => {
  const mutateAsync = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mutateAsync.mockResolvedValue(eventFixture())
    vi.mocked(useExtendEvent).mockReturnValue({
      mutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useExtendEvent>)
  })

  it("submits a preset payload", async () => {
    renderDialog("active")

    await userEvent.click(screen.getByRole("button", { name: "Extend" }))
    await userEvent.click(screen.getByRole("button", { name: "7 days" }))
    await userEvent.click(screen.getByRole("button", { name: "Extend event" }))

    await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith(7))
  })

  it("validates custom days against 1-3650", async () => {
    renderDialog("active")
    await userEvent.click(screen.getByRole("button", { name: "Extend" }))

    const input = screen.getByLabelText("Extend by")
    await userEvent.clear(input)
    await userEvent.type(input, "0")
    await userEvent.click(screen.getByRole("button", { name: "Extend event" }))

    expect(mutateAsync).not.toHaveBeenCalled()
    expect(screen.getByText(/between 1 and 3650/i)).toBeInTheDocument()

    await userEvent.clear(input)
    await userEvent.type(input, "4000")
    await userEvent.click(screen.getByRole("button", { name: "Extend event" }))
    expect(mutateAsync).not.toHaveBeenCalled()

    await userEvent.clear(input)
    await userEvent.type(input, "45")
    await userEvent.click(screen.getByRole("button", { name: "Extend event" }))
    await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith(45))
  })

  it("explains archived events cannot be extended", async () => {
    mutateAsync.mockRejectedValue(
      new ApiError("INVALID_STATUS_TRANSITION", "raw server error", 409)
    )
    renderDialog("active")

    await userEvent.click(screen.getByRole("button", { name: "Extend" }))
    await userEvent.click(screen.getByRole("button", { name: "Extend event" }))

    expect(
      await screen.findByText("Archived events cannot be extended.")
    ).toBeInTheDocument()
    expect(screen.queryByText(/raw server error/)).not.toBeInTheDocument()
  })

  it("renders nothing for archived events", () => {
    const { container } = renderDialog("archived")

    expect(container).toBeEmptyDOMElement()
  })
})
