import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

vi.mock("./api", () => ({
  useDevices: vi.fn(),
  useCreateDevice: vi.fn(),
  useRenameDevice: vi.fn(),
  useRotateDeviceKey: vi.fn(),
  useRevokeDevice: vi.fn(),
}))

vi.mock("@/features/events/api", () => ({
  useEventOptions: vi.fn(),
}))

import { useEventOptions } from "@/features/events/api"

import {
  useCreateDevice,
  useDevices,
  useRenameDevice,
  useRevokeDevice,
  useRotateDeviceKey,
} from "./api"
import { DevicesPanel } from "./devices-panel"

type Device = components["schemas"]["Device"]
type DeviceWithKey = components["schemas"]["DeviceWithKey"]

function device(overrides: Partial<Device> = {}): Device {
  return {
    id: "11111111-1111-1111-1111-111111111111",
    name: "Booth 1",
    keyPrefix: "cpd_abc",
    assignedEventId: null,
    revokedAt: null,
    lastUsedAt: null,
    createdAt: "2026-09-13T10:00:00Z",
    ...overrides,
  }
}

const created: DeviceWithKey = {
  device: device(),
  key: "cpd_live_abcdefghijklmnop",
}

function mockQueries({
  devices,
  events = [],
}: {
  devices: Device[]
  events?: { id: string; name: string; status: string }[]
}) {
  vi.mocked(useDevices).mockReturnValue({
    data: { items: devices },
    isPending: false,
    isError: false,
  } as unknown as ReturnType<typeof useDevices>)
  vi.mocked(useEventOptions).mockReturnValue({
    data: events,
  } as unknown as ReturnType<typeof useEventOptions>)
}

describe("DevicesPanel", () => {
  const createMutate = vi.fn()
  const rotateMutate = vi.fn()
  const revokeMutate = vi.fn()
  const renameMutate = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    createMutate.mockResolvedValue(created)
    rotateMutate.mockResolvedValue(created)
    revokeMutate.mockResolvedValue(undefined)
    renameMutate.mockResolvedValue(device())

    vi.mocked(useCreateDevice).mockReturnValue({
      mutateAsync: createMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useCreateDevice>)
    vi.mocked(useRenameDevice).mockReturnValue({
      mutateAsync: renameMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useRenameDevice>)
    vi.mocked(useRotateDeviceKey).mockReturnValue({
      mutateAsync: rotateMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useRotateDeviceKey>)
    vi.mocked(useRevokeDevice).mockReturnValue({
      mutateAsync: revokeMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useRevokeDevice>)
    mockQueries({ devices: [] })
  })

  it("creates a device and ends in the one-time key dialog", async () => {
    render(<DevicesPanel />)

    expect(screen.getByText("No devices yet")).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: "Add device" }))
    await userEvent.type(screen.getByLabelText("Device name"), "Booth 1")
    await userEvent.click(screen.getByRole("button", { name: "Create device" }))

    await waitFor(() =>
      expect(createMutate).toHaveBeenCalledWith({ name: "Booth 1" })
    )
    expect(await screen.findByText("Device created")).toBeInTheDocument()
  })

  it("lists unknown assigned events with a muted fallback", () => {
    mockQueries({
      devices: [device({ assignedEventId: "missing-event" })],
      events: [{ id: "event-1", name: "Wedding", status: "active" }],
    })

    render(<DevicesPanel />)

    expect(screen.getByText("Unknown event")).toBeInTheDocument()
  })

  it("disables actions on revoked devices", () => {
    mockQueries({
      devices: [device({ revokedAt: "2026-09-13T11:00:00Z" })],
    })

    render(<DevicesPanel />)

    expect(screen.getByText("Revoked")).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Actions for Booth 1" })
    ).toBeDisabled()
  })

  it("confirms before rotating a key", async () => {
    mockQueries({ devices: [device()] })
    render(<DevicesPanel />)

    await userEvent.click(
      screen.getByRole("button", { name: "Actions for Booth 1" })
    )
    await userEvent.click(await screen.findByRole("menuitem", { name: "Rotate key" }))

    expect(
      await screen.findByText("Rotate the key for Booth 1?")
    ).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: "Rotate key" }))

    await waitFor(() => expect(rotateMutate).toHaveBeenCalledTimes(1))
    expect(await screen.findByText("Key rotated")).toBeInTheDocument()
  })

  it("confirms before revoking a device", async () => {
    mockQueries({ devices: [device()] })
    render(<DevicesPanel />)

    await userEvent.click(
      screen.getByRole("button", { name: "Actions for Booth 1" })
    )
    await userEvent.click(await screen.findByRole("menuitem", { name: "Revoke" }))
    await userEvent.click(
      await screen.findByRole("button", { name: "Revoke device" })
    )

    await waitFor(() => expect(revokeMutate).toHaveBeenCalledTimes(1))
  })
})
