import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

const { replace } = vi.hoisted(() => ({ replace: vi.fn() }))

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}))

vi.mock("sonner", () => ({
  toast: { success: vi.fn() },
}))

import { ResetPasswordForm } from "./reset-password-form"

describe("ResetPasswordForm", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080"
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("shows inline mismatch feedback and blocks the submit", async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal("fetch", fetchMock)

    render(<ResetPasswordForm token="reset-token" />)

    await userEvent.type(screen.getByLabelText("New password"), "secret123")
    await userEvent.type(screen.getByLabelText("Confirm password"), "secret124")

    expect(
      await screen.findByText("Passwords do not match yet.")
    ).toBeInTheDocument()
    expect(screen.getByLabelText("Confirm password")).toHaveAttribute(
      "aria-invalid",
      "true"
    )

    await userEvent.click(
      screen.getByRole("button", { name: "Save new password" })
    )

    await waitFor(() => expect(fetchMock).not.toHaveBeenCalled())
  })

  it("clears the mismatch feedback once the passwords match", async () => {
    render(<ResetPasswordForm token="reset-token" />)

    const confirm = screen.getByLabelText("Confirm password")
    await userEvent.type(screen.getByLabelText("New password"), "secret123")
    await userEvent.type(confirm, "secret124")
    expect(
      await screen.findByText("Passwords do not match yet.")
    ).toBeInTheDocument()

    await userEvent.clear(confirm)
    await userEvent.type(confirm, "secret123")

    await waitFor(() =>
      expect(
        screen.queryByText("Passwords do not match yet.")
      ).not.toBeInTheDocument()
    )
    expect(confirm).toHaveAttribute("aria-invalid", "false")
  })

  it("reveals each password field with its own toggle", async () => {
    render(<ResetPasswordForm token="reset-token" />)

    const [newPasswordToggle, confirmToggle] = screen.getAllByRole("button", {
      name: "Show password",
    })
    const newPassword = screen.getByLabelText("New password")
    const confirm = screen.getByLabelText("Confirm password")

    await userEvent.click(newPasswordToggle)
    expect(newPassword).toHaveAttribute("type", "text")
    expect(confirm).toHaveAttribute("type", "password")

    await userEvent.click(confirmToggle)
    expect(confirm).toHaveAttribute("type", "text")
  })

  it("submits the token and new password when the fields match", async () => {
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response(null, { status: 200 })
    )
    vi.stubGlobal("fetch", fetchMock)

    render(<ResetPasswordForm token="reset-token" />)

    await userEvent.type(screen.getByLabelText("New password"), "secret123")
    await userEvent.type(screen.getByLabelText("Confirm password"), "secret123")
    expect(
      screen.queryByText("Passwords do not match yet.")
    ).not.toBeInTheDocument()

    await userEvent.click(
      screen.getByRole("button", { name: "Save new password" })
    )

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    const [url, init] = fetchMock.mock.calls[0]
    expect(String(url)).toBe(
      "http://localhost:18080/api/v1/auth/password/reset-confirm"
    )
    expect(JSON.parse(String(init?.body))).toEqual({
      token: "reset-token",
      password: "secret123",
    })
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/login"))
  })
})
