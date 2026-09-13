import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { DeviceKeyDialog, maskApiKey } from "./device-key-dialog"

type DeviceWithKey = components["schemas"]["DeviceWithKey"]

const result: DeviceWithKey = {
  device: {
    id: "11111111-1111-1111-1111-111111111111",
    name: "Booth 1",
    keyPrefix: "cpd_abc",
    assignedEventId: null,
    revokedAt: null,
    lastUsedAt: null,
    createdAt: "2026-09-13T10:00:00Z",
  },
  key: "cpd_live_abcdefghijklmnop",
}

describe("maskApiKey", () => {
  it("keeps only the edges visible", () => {
    expect(maskApiKey("cpd_live_abcdefghijklmnop")).toBe(
      "cpd_" + "•".repeat(16) + "mnop"
    )
  })

  it("masks short keys entirely", () => {
    expect(maskApiKey("short")).toBe("•••••")
  })
})

describe("DeviceKeyDialog", () => {
  const writeText = vi.fn()

  beforeEach(() => {
    writeText.mockReset()
    writeText.mockResolvedValue(undefined)
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    })
  })

  it("masks the key until revealed", async () => {
    render(<DeviceKeyDialog result={result} action="created" onClose={vi.fn()} />)

    expect(screen.getByText("Device created")).toBeInTheDocument()
    expect(screen.getByText(maskApiKey(result.key))).toBeInTheDocument()
    expect(screen.queryByText(result.key)).toBeNull()

    await userEvent.click(screen.getByRole("button", { name: "Reveal key" }))

    expect(screen.getByText(result.key)).toBeInTheDocument()
  })

  it("copies the full key", async () => {
    render(<DeviceKeyDialog result={result} action="rotated" onClose={vi.fn()} />)

    await userEvent.click(screen.getByRole("button", { name: "Copy key" }))

    expect(writeText).toHaveBeenCalledWith(result.key)
  })

  it("only closes through the acknowledgement", async () => {
    const onClose = vi.fn()
    render(
      <DeviceKeyDialog result={result} action="created" onClose={onClose} />
    )

    await userEvent.keyboard("{Escape}")
    expect(onClose).not.toHaveBeenCalled()

    await userEvent.click(
      screen.getByRole("button", { name: "I've saved it" })
    )
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it("shows the X-Api-Key header example", () => {
    render(<DeviceKeyDialog result={result} action="created" onClose={vi.fn()} />)

    expect(screen.getAllByText(/X-Api-Key/).length).toBeGreaterThan(0)
    expect(screen.getByText(/events\/\{eventId\}\/uploads/)).toBeInTheDocument()
  })
})
