import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { UploadDialog } from "./upload-dialog"

function jpeg(name = "party.jpg") {
  return new File([new Uint8Array([1, 2, 3])], name, { type: "image/jpeg" })
}

function drop(target: Element, dataTransfer: Record<string, unknown>) {
  fireEvent.drop(target, { dataTransfer: { types: ["Files"], ...dataTransfer } })
}

async function openDialog() {
  await userEvent.click(screen.getByRole("button", { name: /upload photos/i }))
  return screen.findByTestId("upload-dropzone")
}

describe("UploadDialog", () => {
  it("opens a large dropzone from the trigger button", async () => {
    render(<UploadDialog onFiles={vi.fn()} />)

    expect(screen.queryByTestId("upload-dropzone")).not.toBeInTheDocument()

    const zone = await openDialog()

    expect(zone).toHaveClass("min-h-80")
  })

  it("queues dropped files and closes the dialog", async () => {
    const onFiles = vi.fn()
    render(<UploadDialog onFiles={onFiles} />)

    const zone = await openDialog()
    const file = jpeg()
    drop(zone, { files: [file] })

    expect(onFiles).toHaveBeenCalledTimes(1)
    expect(onFiles).toHaveBeenCalledWith([file])
    await waitFor(() =>
      expect(screen.queryByTestId("upload-dropzone")).not.toBeInTheDocument()
    )
  })

  it("reports drop errors and keeps the dialog open", async () => {
    const onFiles = vi.fn()
    const onDropError = vi.fn()
    render(<UploadDialog onFiles={onFiles} onDropError={onDropError} />)

    const zone = await openDialog()
    drop(zone, { files: [], items: [] })

    expect(onDropError).toHaveBeenCalledTimes(1)
    expect(onFiles).not.toHaveBeenCalled()
    expect(screen.getByTestId("upload-dropzone")).toBeInTheDocument()
  })
})
