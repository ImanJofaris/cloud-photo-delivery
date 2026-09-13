import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { ApiError } from "@workspace/api-client"

import { UnlockGate } from "./unlock-gate"

describe("UnlockGate", () => {
  it("maps a wrong password to an inline error", () => {
    render(
      <UnlockGate
        eventName="Wedding"
        isPending={false}
        error={new ApiError("UNAUTHORIZED", "Unauthorized", 401)}
        onSubmit={vi.fn()}
      />
    )

    expect(screen.getByRole("alert")).toHaveTextContent(
      "That password is not correct."
    )
    expect(screen.getByLabelText("Password")).toBeInTheDocument()
  })

  it("renders a missing gallery without the password form", () => {
    render(
      <UnlockGate
        eventName="Wedding"
        isPending={false}
        error={new ApiError("EVENT_NOT_FOUND", "Not found", 404)}
        onSubmit={vi.fn()}
      />
    )

    expect(
      screen.getByText("This gallery is no longer available.")
    ).toBeInTheDocument()
    expect(screen.queryByLabelText("Password")).toBeNull()
  })

  it("submits the entered password", async () => {
    const onSubmit = vi.fn()
    render(
      <UnlockGate
        eventName="Wedding"
        isPending={false}
        error={null}
        onSubmit={onSubmit}
      />
    )

    await userEvent.type(screen.getByLabelText("Password"), "open-sesame")
    await userEvent.click(
      screen.getByRole("button", { name: "Unlock gallery" })
    )

    expect(onSubmit).toHaveBeenCalledWith("open-sesame")
  })
})
