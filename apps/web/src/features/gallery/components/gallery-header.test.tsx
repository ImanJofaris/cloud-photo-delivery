import { render, screen, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"

import { GalleryHeader } from "./gallery-header"
import { createQueryWrapper, jsonResponse, requestUrl } from "../test-utils"
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
  afterEach(() => {
    vi.unstubAllGlobals()
  })

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

  it("renders the profile image as the banner when there is no cover photo", () => {
    const { container } = render(
      <GalleryHeader
        slug="wedding"
        viewable
        event={event({
          branding: {
            businessName: "Booth Co",
            profileImageUrl: "https://cdn.example/profile.png",
          },
        })}
      />,
      { wrapper: createQueryWrapper() }
    )

    expect(container.querySelector("header img")).toHaveAttribute(
      "src",
      "https://cdn.example/profile.png"
    )
  })

  it("prefers the cover photo over the profile image", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080"
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = new URL(requestUrl(input))
        const variant = url.searchParams.get("variant") ?? "thumbnail"
        return jsonResponse({
          data: { url: `https://r2.test/${variant}.jpg`, expiresIn: 300 },
          error: null,
        })
      })
    )

    const { container } = render(
      <GalleryHeader
        slug="wedding"
        viewable
        event={event({
          coverPhotoId: "p1",
          branding: { profileImageUrl: "https://cdn.example/profile.png" },
        })}
      />,
      { wrapper: createQueryWrapper() }
    )

    await waitFor(() =>
      expect(container.querySelector("header img")).toHaveAttribute(
        "src",
        "https://r2.test/large.jpg"
      )
    )
  })
})
