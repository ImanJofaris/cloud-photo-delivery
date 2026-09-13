import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import * as React from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { GalleryEmpty } from "./gallery-empty"
import { PhotoGrid } from "./photo-grid"
import { createQueryWrapper, jsonResponse, requestUrl } from "../test-utils"
import type { GalleryPhoto } from "../types"

function photo(overrides: Partial<GalleryPhoto> = {}): GalleryPhoto {
  return {
    id: "p1",
    width: 800,
    height: 600,
    variants: ["thumbnail"],
    ...overrides,
  }
}

function renderGrid(
  overrides: Partial<React.ComponentProps<typeof PhotoGrid>> = {}
) {
  const handlers = {
    onLoadMore: vi.fn(),
    onOpen: vi.fn(),
  }
  render(
    <PhotoGrid
      slug="wedding"
      photos={[photo()]}
      eventName="Wedding"
      hasNextPage={false}
      isFetchingNextPage={false}
      {...handlers}
      {...overrides}
    />,
    { wrapper: createQueryWrapper() }
  )
  return handlers
}

describe("PhotoGrid", () => {
  beforeEach(() => {
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
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("renders tiles with aspect ratios from the photo dimensions", async () => {
    renderGrid({
      photos: [
        photo(),
        photo({ id: "p2", width: 400, height: 500 }),
      ],
    })

    const first = screen.getByRole("button", {
      name: "Open Wedding photo 1",
    })
    expect(first.getAttribute("style")).toContain("aspect-ratio")
    expect(first.getAttribute("style")).toContain("800 / 600")

    const image = await screen.findByRole("img", {
      name: "Wedding photo 1",
    })
    expect(image).toHaveAttribute("src", "https://r2.test/thumbnail.jpg")
  })

  it("opens the viewer for the tapped tile", async () => {
    const { onOpen } = renderGrid({
      photos: [photo(), photo({ id: "p2" })],
    })

    await userEvent.click(
      screen.getByRole("button", { name: "Open Wedding photo 2" })
    )

    expect(onOpen).toHaveBeenCalledWith(1)
  })

  it("loads more from the accessible button", async () => {
    const { onLoadMore } = renderGrid({ hasNextPage: true })

    await waitFor(() => expect(onLoadMore).toHaveBeenCalled())
    onLoadMore.mockClear()

    await userEvent.click(screen.getByRole("button", { name: "Load more" }))

    expect(onLoadMore).toHaveBeenCalledTimes(1)
  })

  it("disables the load more control while fetching", () => {
    renderGrid({ hasNextPage: true, isFetchingNextPage: true })

    expect(screen.getByRole("button", { name: "Loading..." })).toBeDisabled()
  })
})

describe("GalleryEmpty", () => {
  it("points guests at the event once photos arrive", () => {
    render(<GalleryEmpty eventName="Wedding" />)

    expect(screen.getByText("No photos yet")).toBeInTheDocument()
    expect(screen.getByText(/Photos from Wedding/)).toBeInTheDocument()
  })
})
