import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError } from "@workspace/api-client"

const { push, toastSuccess, toastError } = vi.hoisted(() => ({
  push: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}))

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}))

vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: toastError },
}))

vi.mock("./api", () => ({
  useArchiveEvent: vi.fn(),
  useDeleteEvent: vi.fn(),
}))

import { useArchiveEvent, useDeleteEvent } from "./api"
import { DangerZone } from "./danger-zone"

describe("DangerZone", () => {
  const archiveMutateAsync = vi.fn()
  const deleteMutateAsync = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    archiveMutateAsync.mockResolvedValue({})
    deleteMutateAsync.mockResolvedValue(undefined)
    vi.mocked(useArchiveEvent).mockReturnValue({
      mutateAsync: archiveMutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useArchiveEvent>)
    vi.mocked(useDeleteEvent).mockReturnValue({
      mutateAsync: deleteMutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useDeleteEvent>)
  })

  function renderZone(status = "active") {
    return render(
      <DangerZone
        eventId="event-1"
        eventName="Aina & Haziq Wedding"
        status={status}
      />
    )
  }

  it("archives the event", async () => {
    renderZone()

    await userEvent.click(screen.getByRole("button", { name: "Archive" }))

    await waitFor(() => expect(archiveMutateAsync).toHaveBeenCalledWith())
    expect(toastSuccess).toHaveBeenCalledWith("Event archived")
  })

  it("cannot archive an already archived event", () => {
    renderZone("archived")

    expect(screen.getByRole("button", { name: "Archive" })).toBeDisabled()
  })

  it("requires the exact event name before deleting", async () => {
    renderZone()

    const confirm = screen.getByLabelText("Event name")
    const deleteButton = screen.getByRole("button", { name: "Delete event" })
    expect(deleteButton).toBeDisabled()

    await userEvent.type(confirm, "Aina & Haziq")
    expect(deleteButton).toBeDisabled()

    await userEvent.type(confirm, " Wedding")
    expect(deleteButton).toBeEnabled()

    await userEvent.click(deleteButton)

    await waitFor(() => expect(deleteMutateAsync).toHaveBeenCalledWith())
    expect(push).toHaveBeenCalledWith("/events")
    expect(toastSuccess).toHaveBeenCalledWith("Event deleted")
  })

  it("maps delete failures without leaking server text", async () => {
    deleteMutateAsync.mockRejectedValue(
      new ApiError("EVENT_NOT_FOUND", "raw server error", 404)
    )
    renderZone()

    await userEvent.type(
      screen.getByLabelText("Event name"),
      "Aina & Haziq Wedding"
    )
    await userEvent.click(screen.getByRole("button", { name: "Delete event" }))

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("This event no longer exists.")
    )
    expect(push).not.toHaveBeenCalled()
  })
})
