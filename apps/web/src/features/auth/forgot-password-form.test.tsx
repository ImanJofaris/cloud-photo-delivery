import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { ForgotPasswordForm } from "./forgot-password-form"

describe("ForgotPasswordForm", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080"
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("keeps the form visible and offers a resend after sending", async () => {
    const fetchMock = vi.fn(async () => new Response(null, { status: 200 }))
    vi.stubGlobal("fetch", fetchMock)

    render(<ForgotPasswordForm />)

    await userEvent.type(screen.getByLabelText("Email"), "hello@pixelparty.my")
    await userEvent.click(
      screen.getByRole("button", { name: "Send reset link" })
    )

    expect(await screen.findByRole("status")).toHaveTextContent(
      "If that email is registered, a reset link is on its way."
    )
    expect(screen.getByLabelText("Email")).toHaveValue("hello@pixelparty.my")

    await userEvent.click(screen.getByRole("button", { name: "Resend link" }))

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2))
  })
})
