import { render, screen } from "@testing-library/react"
import type { ComponentProps } from "react"
import { describe, expect, it } from "vitest"

import { GuestPreviewCard } from "./guest-preview"

function renderPreview(
  overrides: Partial<ComponentProps<typeof GuestPreviewCard>> = {}
) {
  return render(
    <GuestPreviewCard
      eventName="Aina & Haziq Wedding"
      eventDate="2026-10-04"
      location="Dewan Perdana Felda, KL"
      photoCount={486}
      visibility="public"
      allowDownload
      allowOriginalDownload={false}
      watermarkEnabled={false}
      primaryColor={null}
      businessName="Pixel Party"
      {...overrides}
    />
  )
}

describe("GuestPreviewCard", () => {
  it("shows the event, its meta, and the photo count", () => {
    renderPreview()

    expect(screen.getByText("Guest preview")).toBeInTheDocument()
    expect(screen.getByText("Aina & Haziq Wedding")).toBeInTheDocument()
    expect(screen.getByText(/Dewan Perdana Felda, KL/)).toBeInTheDocument()
    expect(screen.getByText("486 photos")).toBeInTheDocument()
    expect(screen.getByText("PP")).toBeInTheDocument()
  })

  it("shows the download action only when downloads are allowed", () => {
    const { unmount } = renderPreview()
    expect(screen.getByText("Download all")).toBeInTheDocument()
    expect(screen.getByText("Downloads on")).toBeInTheDocument()
    unmount()

    renderPreview({ allowDownload: false })
    expect(screen.queryByText("Download all")).not.toBeInTheDocument()
    expect(screen.getByText("Downloads off")).toBeInTheDocument()
  })

  it("marks originals only when both download switches are on", () => {
    const { unmount } = renderPreview({ allowOriginalDownload: true })
    expect(screen.getByText("Originals")).toBeInTheDocument()
    unmount()

    renderPreview({ allowDownload: false, allowOriginalDownload: false })
    expect(screen.queryByText("Originals")).not.toBeInTheDocument()
  })

  it("marks the gallery when a password is required", () => {
    renderPreview({ visibility: "password" })
    expect(screen.getByText("Password")).toBeInTheDocument()
  })

  it("shows the watermark marker only when the watermark is on", () => {
    const { unmount } = renderPreview({ watermarkEnabled: true })
    expect(screen.getByText("WATERMARK")).toBeInTheDocument()
    unmount()

    renderPreview({ watermarkEnabled: false })
    expect(screen.queryByText("WATERMARK")).not.toBeInTheDocument()
  })

  it("uses the branding primary color when provided", () => {
    renderPreview({ primaryColor: "#c2477f" })

    expect(screen.getByText("PP").style.backgroundColor).toBe(
      "rgb(194, 71, 127)"
    )
  })

  it("falls back to the default accent without a branding color", () => {
    renderPreview()

    const initials = screen.getByText("PP")
    expect(initials.style.backgroundColor).toBe("")
    expect(initials).toHaveClass("bg-primary")
  })
})
