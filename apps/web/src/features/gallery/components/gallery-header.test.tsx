import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { GalleryHeader } from "./gallery-header"
import { createQueryWrapper } from "../test-utils"
import type { GalleryEvent } from "../types"

function event(overrides: Partial<GalleryEvent> = {}): GalleryEvent {
  return {
    name: "Aisyah & Danial",
    date: "2026-08-01",
    location: "Kuala Lumpur",
    description: "Reception",
    visibility: "public",
    allowDownload: true,
    allowOriginalDownload: false,
    photoCount: 12,
    coverPhotoId: null,
    requiresUnlock: false,
    branding: null,
    ...overrides,
  }
}

describe("GalleryHeader", () => {
  it("renders branding when present", () => {
    render(
      <GalleryHeader
        slug="wedding"
        viewable
        event={event({
          branding: {
            businessName: "Booth Co",
            contactEmail: "hello@example.com",
            contactPhone: "+60123456789",
            websiteUrl: "https://booth.example",
            primaryColor: "#112233",
          },
        })}
      />,
      { wrapper: createQueryWrapper() }
    )

    expect(
      screen.getByRole("heading", { name: "Aisyah & Danial" })
    ).toBeInTheDocument()
    expect(screen.getByText("Booth Co")).toBeInTheDocument()
    expect(
      screen.getByRole("link", { name: /hello@example\.com/ })
    ).toHaveAttribute("href", "mailto:hello@example.com")
    expect(
      screen.getByRole("link", { name: /\+60123456789/ })
    ).toHaveAttribute("href", "tel:+60123456789")
    expect(screen.getByRole("link", { name: /booth\.example/ })).toHaveAttribute(
      "href",
      "https://booth.example"
    )
  })

  it("falls back to event metadata when branding is null", () => {
    render(<GalleryHeader slug="wedding" viewable event={event()} />, {
      wrapper: createQueryWrapper(),
    })

    expect(
      screen.getByRole("heading", { name: "Aisyah & Danial" })
    ).toBeInTheDocument()
    expect(screen.getByText("Kuala Lumpur")).toBeInTheDocument()
    expect(screen.getByText("Reception")).toBeInTheDocument()
    expect(screen.queryByRole("link")).toBeNull()
  })
})
