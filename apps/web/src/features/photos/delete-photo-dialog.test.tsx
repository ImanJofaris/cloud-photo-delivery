import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { DeletePhotoDialog } from "./delete-photo-dialog"

type Photo = components["schemas"]["Photo"]

function photo(id: string): Photo {
  return {
    id,
    filename: `${id}.jpg`,
    mimeType: "image/jpeg",
    fileSize: 1024,
    width: 1200,
    height: 800,
    status: "READY",
    variants: ["thumbnail"],
    errorMessage: null,
    createdAt: "2026-10-01T00:00:00Z",
  }
}

describe("DeletePhotoDialog", () => {
  it("names the photo when deleting one", () => {
    render(
      <DeletePhotoDialog
        photos={[photo("p1")]}
        pending={false}
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
      />
    )

    expect(screen.getByText("Delete photo?")).toBeInTheDocument()
    expect(screen.getByText(/"p1\.jpg"/)).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Delete photo" })
    ).toBeInTheDocument()
  })

  it("counts the photos and confirms a bulk delete", async () => {
    const onConfirm = vi.fn()
    render(
      <DeletePhotoDialog
        photos={[photo("p1"), photo("p2")]}
        pending={false}
        onOpenChange={vi.fn()}
        onConfirm={onConfirm}
      />
    )

    expect(screen.getByText("Delete 2 photos?")).toBeInTheDocument()
    await userEvent.click(
      screen.getByRole("button", { name: "Delete 2 photos" })
    )
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it("blocks both actions while deleting", () => {
    render(
      <DeletePhotoDialog
        photos={[photo("p1")]}
        pending
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
      />
    )

    expect(screen.getByRole("button", { name: "Deleting..." })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled()
  })
})
