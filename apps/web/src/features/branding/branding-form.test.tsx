import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

vi.mock("./api", () => ({
  useBranding: vi.fn(),
  useUpdateBranding: vi.fn(),
  useCreateBrandingAssetUpload: vi.fn(),
}))

vi.mock("@/features/billing/api", () => ({
  useSubscription: vi.fn(),
}))

import { useSubscription } from "@/features/billing/api"

import { useBranding, useCreateBrandingAssetUpload, useUpdateBranding } from "./api"
import { BrandingForm } from "./branding-form"

type Branding = components["schemas"]["Branding"]

const branding: Branding = {
  businessName: "Booth Co",
  primaryColor: "#112233",
  secondaryColor: null,
  contactEmail: "hello@booth.example",
  contactPhone: "+60123",
  websiteUrl: null,
  logoUrl: null,
  profileImageUrl: null,
  updatedAt: "2026-09-13T10:00:00Z",
}

describe("BrandingForm", () => {
  const updateMutate = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    updateMutate.mockImplementation(async (patch: Partial<Branding>) => ({
      ...branding,
      ...patch,
    }))
    vi.mocked(useBranding).mockReturnValue({
      data: branding,
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useBranding>)
    vi.mocked(useUpdateBranding).mockReturnValue({
      mutateAsync: updateMutate,
      isPending: false,
    } as unknown as ReturnType<typeof useUpdateBranding>)
    vi.mocked(useCreateBrandingAssetUpload).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useCreateBrandingAssetUpload>)
    vi.mocked(useSubscription).mockReturnValue({
      data: { plan: { limits: { branding: true } } },
      isPending: false,
    } as unknown as ReturnType<typeof useSubscription>)
  })

  it("shows the upgrade card when the plan has no branding", () => {
    vi.mocked(useSubscription).mockReturnValue({
      data: { plan: { limits: { branding: false } } },
      isPending: false,
    } as unknown as ReturnType<typeof useSubscription>)

    render(<BrandingForm />)

    expect(
      screen.getByText("Custom branding is a Starter feature")
    ).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "View plans" })).toHaveAttribute(
      "href",
      "/billing"
    )
    expect(screen.queryByLabelText("Business name")).not.toBeInTheDocument()
  })

  it("patches only the changed field and omits assets", async () => {
    render(<BrandingForm />)

    const name = await screen.findByLabelText("Business name")
    await userEvent.clear(name)
    await userEvent.type(name, "New Booth")
    await userEvent.click(screen.getByRole("button", { name: "Save branding" }))

    await waitFor(() =>
      expect(updateMutate).toHaveBeenCalledWith({ businessName: "New Booth" })
    )
  })

  it("sends an empty string to clear an existing field", async () => {
    render(<BrandingForm />)

    const phone = await screen.findByLabelText("Contact phone")
    await userEvent.clear(phone)
    await userEvent.click(screen.getByRole("button", { name: "Save branding" }))

    await waitFor(() =>
      expect(updateMutate).toHaveBeenCalledWith({ contactPhone: "" })
    )
  })

  it("blocks invalid email values before calling the API", async () => {
    render(<BrandingForm />)

    const email = await screen.findByLabelText("Contact email")
    await userEvent.clear(email)
    await userEvent.type(email, "not-an-email")
    await userEvent.click(screen.getByRole("button", { name: "Save branding" }))

    expect(
      await screen.findByText("Enter a valid email address.")
    ).toBeInTheDocument()
    expect(updateMutate).not.toHaveBeenCalled()
  })
})
