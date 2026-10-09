import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>()
  return {
    ...actual,
    useEvent: vi.fn(),
    useEventSettings: vi.fn(),
    useUpdateEventSettings: vi.fn(),
  }
})

vi.mock("@/features/branding/api", () => ({ useBranding: vi.fn() }))
vi.mock("@/features/billing/api", () => ({ useSubscription: vi.fn() }))

import { useBranding } from "@/features/branding/api"
import { useSubscription } from "@/features/billing/api"

import { useEvent, useEventSettings, useUpdateEventSettings } from "./api"
import { EventSettingsForm } from "./settings-form"
import type { EventSettingsValues } from "./schema"

type Event = components["schemas"]["Event"]
type EventSettings = components["schemas"]["EventSettings"]

const event: Event = {
  id: "event-1",
  name: "Aina & Haziq Wedding",
  slug: "aina-haziq-wedding",
  status: "active",
  eventDate: "2026-10-04",
  location: "Dewan Perdana Felda, KL",
  storageBytes: 0,
  photoCount: 486,
  guestCount: 0,
  expiresAt: "2026-12-01T00:00:00Z",
  createdAt: "2026-09-01T00:00:00Z",
  updatedAt: "2026-09-01T00:00:00Z",
}

const settings: EventSettings = {
  eventId: "event-1",
  visibility: "public",
  passwordProtected: false,
  allowDownload: true,
  allowOriginalDownload: false,
  watermarkEnabled: false,
  updatedAt: "2026-09-01T00:00:00Z",
}

const branding = {
  businessName: "Pixel Party",
  primaryColor: "#c2477f",
  secondaryColor: null,
  contactEmail: null,
  contactPhone: null,
  websiteUrl: null,
  logoUrl: null,
  profileImageUrl: null,
  updatedAt: null,
}

const defaults: EventSettingsValues = {
  visibility: "public",
  password: "",
  allowDownload: true,
  allowOriginalDownload: false,
  watermarkEnabled: false,
}

describe("EventSettingsForm", () => {
  const mutateAsync = vi.fn()

  function mockHooks() {
    vi.mocked(useUpdateEventSettings).mockReturnValue({
      mutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useUpdateEventSettings>)
    vi.mocked(useEventSettings).mockReturnValue({
      data: settings,
    } as unknown as ReturnType<typeof useEventSettings>)
    vi.mocked(useEvent).mockReturnValue({
      data: event,
    } as unknown as ReturnType<typeof useEvent>)
    vi.mocked(useBranding).mockReturnValue({
      data: branding,
    } as unknown as ReturnType<typeof useBranding>)
    vi.mocked(useSubscription).mockReturnValue({
      data: { plan: { limits: { branding: true } } },
    } as unknown as ReturnType<typeof useSubscription>)
  }

  function renderForm(overrides: Partial<typeof defaults> = {}) {
    return render(
      <EventSettingsForm
        eventId="event-1"
        defaults={{ ...defaults, ...overrides }}
      />
    )
  }

  beforeEach(() => {
    vi.clearAllMocks()
    mutateAsync.mockResolvedValue(settings)
    mockHooks()
  })

  it("renders the access, permissions, and preview cards", () => {
    renderForm()

    expect(screen.getByText("Gallery access")).toBeInTheDocument()
    expect(screen.getByText("Guest permissions")).toBeInTheDocument()
    expect(screen.getByText("Guest preview")).toBeInTheDocument()
    expect(
      screen.getByRole("switch", { name: "Allow downloads" })
    ).toBeChecked()
    expect(
      screen.getByRole("switch", { name: "Original downloads" })
    ).not.toBeChecked()
    expect(screen.getByRole("switch", { name: "Watermark" })).not.toBeChecked()
    expect(screen.getByLabelText("Gallery password")).toBeDisabled()
  })

  it("enables the password field in password mode and requires it", async () => {
    renderForm()

    await userEvent.click(screen.getByRole("button", { name: "Password" }))

    const password = screen.getByLabelText("Gallery password")
    expect(password).toBeEnabled()

    await userEvent.click(screen.getByRole("button", { name: "Save settings" }))

    expect(
      await screen.findByText("Enter a password for password protected events")
    ).toBeInTheDocument()
    expect(mutateAsync).not.toHaveBeenCalled()
  })

  it("keeps an existing password when the field is left blank", async () => {
    vi.mocked(useEventSettings).mockReturnValue({
      data: { ...settings, visibility: "password", passwordProtected: true },
    } as unknown as ReturnType<typeof useEventSettings>)
    renderForm({ visibility: "password" })

    await userEvent.click(screen.getByRole("button", { name: "Save settings" }))

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        visibility: "password",
        allowDownload: true,
        allowOriginalDownload: false,
        watermarkEnabled: false,
      })
    )
  })

  it("toggles password visibility", async () => {
    renderForm()

    await userEvent.click(screen.getByRole("button", { name: "Password" }))
    const password = screen.getByLabelText("Gallery password")
    await userEvent.type(password, "open-sesame")
    expect(password).toHaveAttribute("type", "password")

    await userEvent.click(screen.getByRole("button", { name: "Show password" }))
    expect(password).toHaveAttribute("type", "text")

    await userEvent.click(screen.getByRole("button", { name: "Hide password" }))
    expect(password).toHaveAttribute("type", "password")
  })

  it("submits the current settings", async () => {
    renderForm()

    await userEvent.click(screen.getByRole("button", { name: "Save settings" }))

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        visibility: "public",
        allowDownload: true,
        allowOriginalDownload: false,
        watermarkEnabled: false,
      })
    )
  })

  it("reflects the download switches in the guest preview", async () => {
    renderForm()

    expect(screen.getByText("Download all")).toBeInTheDocument()
    expect(screen.getByText("Downloads on")).toBeInTheDocument()

    await userEvent.click(
      screen.getByRole("switch", { name: "Allow downloads" })
    )

    expect(screen.queryByText("Download all")).not.toBeInTheDocument()
    expect(screen.getByText("Downloads off")).toBeInTheDocument()
  })

  it("blocks original downloads without downloads", async () => {
    renderForm()

    await userEvent.click(
      screen.getByRole("switch", { name: "Allow downloads" })
    )
    await userEvent.click(
      screen.getByRole("switch", { name: "Original downloads" })
    )
    await userEvent.click(screen.getByRole("button", { name: "Save settings" }))

    expect(
      await screen.findByText(
        "Enable downloads before allowing original downloads"
      )
    ).toBeInTheDocument()
    expect(mutateAsync).not.toHaveBeenCalled()
  })

  it("uses the branding color only when the plan allows branding", () => {
    const { unmount } = renderForm()
    expect(screen.getByText("PP").style.backgroundColor).toBe(
      "rgb(194, 71, 127)"
    )
    unmount()

    vi.mocked(useSubscription).mockReturnValue({
      data: { plan: { limits: { branding: false } } },
    } as unknown as ReturnType<typeof useSubscription>)
    renderForm()
    const fallbackInitials = screen.getByText("AH")
    expect(fallbackInitials.style.backgroundColor).toBe("")
    expect(fallbackInitials).toHaveClass("bg-primary")
  })
})
