import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import { UploadDropzone } from "./upload-dropzone"

function jpeg(name = "party.jpg") {
  return new File([new Uint8Array([1, 2, 3])], name, { type: "image/jpeg" })
}

function drop(target: Element, dataTransfer: Record<string, unknown>) {
  fireEvent.drop(target, { dataTransfer: { types: ["Files"], ...dataTransfer } })
}

function fileEntry(file: File): FileSystemEntry {
  return {
    isFile: true,
    isDirectory: false,
    name: file.name,
    fullPath: file.name,
    file: (resolve: (value: File) => void) => resolve(file),
  } as unknown as FileSystemEntry
}

function directoryEntry(children: FileSystemEntry[]): FileSystemEntry {
  let served = false
  return {
    isFile: false,
    isDirectory: true,
    name: "session",
    fullPath: "session",
    createReader: () => ({
      readEntries: (resolve: (entries: FileSystemEntry[]) => void) => {
        if (served) {
          resolve([])
          return
        }
        served = true
        resolve(children)
      },
    }),
  } as unknown as FileSystemEntry
}

describe("UploadDropzone", () => {
  it("calls onFiles with dropped files", () => {
    const onFiles = vi.fn()
    render(<UploadDropzone onFiles={onFiles} />)

    const file = jpeg()
    drop(screen.getByTestId("upload-dropzone"), { files: [file] })

    expect(onFiles).toHaveBeenCalledTimes(1)
    expect(onFiles).toHaveBeenCalledWith([file])
  })

  it("calls onFiles when the drop lands on a child element", () => {
    const onFiles = vi.fn()
    render(<UploadDropzone onFiles={onFiles} />)

    const file = jpeg()
    drop(screen.getByRole("button", { name: /choose photos/i }), {
      files: [file],
    })

    expect(onFiles).toHaveBeenCalledWith([file])
  })

  it("ignores drops while disabled", () => {
    const onFiles = vi.fn()
    render(<UploadDropzone onFiles={onFiles} disabled />)

    drop(screen.getByTestId("upload-dropzone"), { files: [jpeg()] })

    expect(onFiles).not.toHaveBeenCalled()
  })

  it("highlights while a file drag is over the zone and clears on leave", () => {
    render(<UploadDropzone onFiles={vi.fn()} />)
    const zone = screen.getByTestId("upload-dropzone")

    fireEvent.dragEnter(zone)
    expect(zone).toHaveClass("border-primary")

    fireEvent.dragLeave(zone)
    expect(zone).not.toHaveClass("border-primary")
  })

  it("keeps the highlight when the drag moves onto a child", () => {
    render(<UploadDropzone onFiles={vi.fn()} />)
    const zone = screen.getByTestId("upload-dropzone")
    const button = screen.getByRole("button", { name: /choose photos/i })

    fireEvent.dragEnter(zone)
    fireEvent.dragEnter(button)
    fireEvent.dragLeave(zone)

    expect(zone).toHaveClass("border-primary")

    fireEvent.dragLeave(button)
    expect(zone).not.toHaveClass("border-primary")
  })

  it("reports a drop that carries no files", () => {
    const onFiles = vi.fn()
    const onDropError = vi.fn()
    render(<UploadDropzone onFiles={onFiles} onDropError={onDropError} />)

    drop(screen.getByTestId("upload-dropzone"), { files: [], items: [] })

    expect(onFiles).not.toHaveBeenCalled()
    expect(onDropError).toHaveBeenCalledTimes(1)
  })

  it("reads photos out of a dropped folder", async () => {
    const onFiles = vi.fn()
    const file = jpeg("shot.jpg")
    const directory = directoryEntry([fileEntry(file)])
    const item = { webkitGetAsEntry: () => directory }

    render(<UploadDropzone onFiles={onFiles} />)
    drop(screen.getByTestId("upload-dropzone"), {
      files: [],
      items: [item],
    })

    await waitFor(() => expect(onFiles).toHaveBeenCalledWith([file]))
  })

  it("reports a folder without any photos", async () => {
    const onFiles = vi.fn()
    const onDropError = vi.fn()
    const notAPhoto = new File([new Uint8Array([1])], "notes.txt", {
      type: "text/plain",
    })
    const directory = directoryEntry([fileEntry(notAPhoto)])
    const item = { webkitGetAsEntry: () => directory }

    render(<UploadDropzone onFiles={onFiles} onDropError={onDropError} />)
    drop(screen.getByTestId("upload-dropzone"), {
      files: [],
      items: [item],
    })

    await waitFor(() => expect(onDropError).toHaveBeenCalledTimes(1))
    expect(onFiles).not.toHaveBeenCalled()
  })
})
