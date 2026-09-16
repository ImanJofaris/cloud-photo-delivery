import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError, type components } from "@workspace/api-client"

vi.mock("./api", () => ({
  useTransitionEvent: vi.fn(),
}))

import { useTransitionEvent } from "./api"
import { EventStatusCard } from "./status-actions"

type EventStatus = components["schemas"]["EventStatus"]

describe("EventStatusCard", () => {
  const mutateAsync = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useTransitionEvent).mockReturnValue({
      mutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useTransitionEvent>)
  })

  function renderCard(status: EventStatus) {
    return render(<EventStatusCard eventId="event-1" status={status} />)
  }

  it("starts an upcoming event", async () => {
    mutateAsync.mockResolvedValue({})
    renderCard("upcoming")

    await userEvent.click(
      screen.getByRole("button", { name: "Mark active" })
    )

    await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith("active"))
  })

  it("completes an active event", async () => {
    mutateAsync.mockResolvedValue({})
    renderCard("active")

    await userEvent.click(
      screen.getByRole("button", { name: "Mark completed" })
    )

    await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith("completed"))
  })

  it.each<EventStatus>(["completed", "archived", "expired"])(
    "renders nothing for %s events",
    (status) => {
      const { container } = renderCard(status)

      expect(container).toBeEmptyDOMElement()
    }
  )

  it("links to billing when the plan limit blocks activation", async () => {
    mutateAsync.mockRejectedValue(
      new ApiError("PLAN_LIMIT_REACHED", "raw server error", 402)
    )
    renderCard("upcoming")

    await userEvent.click(
      screen.getByRole("button", { name: "Mark active" })
    )

    expect(
      await screen.findByText("You have reached your plan's event limit.")
    ).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "View plans" })).toHaveAttribute(
      "href",
      "/billing"
    )
    expect(screen.queryByText(/raw server error/)).not.toBeInTheDocument()
  })

  it("maps invalid transitions without leaking server text", async () => {
    mutateAsync.mockRejectedValue(
      new ApiError("INVALID_STATUS_TRANSITION", "raw server error", 409)
    )
    renderCard("active")

    await userEvent.click(
      screen.getByRole("button", { name: "Mark completed" })
    )

    expect(
      await screen.findByText(
        "That action is not allowed for the event's current status."
      )
    ).toBeInTheDocument()
    expect(screen.queryByText(/raw server error/)).not.toBeInTheDocument()
    expect(
      screen.queryByRole("button", { name: "View plans" })
    ).not.toBeInTheDocument()
  })
})
