import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import * as React from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { PhotoViewer } from "./photo-viewer"
import { createQueryWrapper, jsonResponse, requestUrl } from "../test-utils"
import type { GalleryPhoto } from "../types"

const photos: GalleryPhoto[] = [
  { id: "p1", width: 800, height: 600, variants: ["large"] },
  { id: "p2", width: 800, height: 600, variants: ["large"] },
  { id: "p3", width: 800, height: 600, variants: ["large"] },
]

function renderViewer(
  overrides: Partial<React.ComponentProps<typeof PhotoViewer>> = {}
) {
  const handlers = {
    onClose: vi.fn(),
    onIndexChange: vi.fn(),
    onLoadMore: vi.fn(),
  }
  render(
    <PhotoViewer
      slug="wedding"
      photos={photos}
      index={0}
      eventName="Wedding"
      allowDownload={false}
      allowOriginalDownload={false}
      hasNextPage={false}
      {...handlers}
      {...overrides}
    />,
    { wrapper: createQueryWrapper() }
  )
  return handlers
}

describe("PhotoViewer", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080"
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = new URL(requestUrl(input))
        const variant = url.searchParams.get("variant") ?? "large"
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

  it("hides download controls for view-only events", () => {
    renderViewer({ allowDownload: false })

    expect(screen.queryByRole("button", { name: "Download" })).toBeNull()
    expect(screen.queryByRole("button", { name: "Original" })).toBeNull()
  })

  it("offers the derivative download when downloads are allowed", () => {
    renderViewer({ allowDownload: true, allowOriginalDownload: false })

    expect(screen.getByRole("button", { name: "Download" })).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Original" })).toBeNull()
  })

  it("offers the original download only when originals are allowed", () => {
    renderViewer({ allowDownload: true, allowOriginalDownload: true })

    expect(screen.getByRole("button", { name: "Download" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Original" })).toBeInTheDocument()
  })

  it("navigates with the next arrow", async () => {
    const { onIndexChange } = renderViewer({ index: 0 })

    await userEvent.click(screen.getByRole("button", { name: "Next photo" }))

    expect(onIndexChange).toHaveBeenCalledWith(1)
  })

  it("navigates with the keyboard and closes on escape", () => {
    const { onClose, onIndexChange } = renderViewer({ index: 1 })
    const dialog = screen.getByRole("dialog")

    fireEvent.keyDown(dialog, { key: "ArrowLeft" })
    expect(onIndexChange).toHaveBeenCalledWith(0)

    fireEvent.keyDown(dialog, { key: "ArrowRight" })
    expect(onIndexChange).toHaveBeenCalledWith(2)

    fireEvent.keyDown(dialog, { key: "Escape" })
    expect(onClose).toHaveBeenCalled()
  })

  it("keeps focus inside the dialog", async () => {
    renderViewer()
    const dialog = screen.getByRole("dialog")

    await waitFor(() =>
      expect(dialog.contains(document.activeElement)).toBe(true)
    )
  })
})
