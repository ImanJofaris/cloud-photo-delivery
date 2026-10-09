import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it } from "vitest"

import { Label } from "@workspace/ui/components/label"

import { PasswordInput } from "./password-input"

function renderPasswordInput() {
  render(
    <>
      <Label htmlFor="password">Password</Label>
      <PasswordInput id="password" autoComplete="current-password" />
    </>
  )

  return screen.getByLabelText("Password")
}

describe("PasswordInput", () => {
  it("starts masked with an accessible show control", () => {
    const input = renderPasswordInput()

    expect(input).toHaveAttribute("type", "password")
    expect(
      screen.getByRole("button", { name: "Show password" })
    ).toHaveAttribute("type", "button")
  })

  it("toggles between hidden and visible", async () => {
    const input = renderPasswordInput()

    await userEvent.click(screen.getByRole("button", { name: "Show password" }))
    expect(input).toHaveAttribute("type", "text")
    expect(screen.getByRole("button", { name: "Hide password" })).toBeVisible()

    await userEvent.click(screen.getByRole("button", { name: "Hide password" }))
    expect(input).toHaveAttribute("type", "password")
    expect(screen.getByRole("button", { name: "Show password" })).toBeVisible()
  })
})
