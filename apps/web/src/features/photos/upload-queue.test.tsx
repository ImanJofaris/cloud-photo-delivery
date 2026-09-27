import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { UploadQueueList } from "./upload-queue"
import type { UploadItem } from "./uploader"

function item(overrides: Partial<UploadItem> = {}): UploadItem {
  return {
    key: "k1",
    filename: "a.jpg",
    size: 1024,
    mimeType: "image/jpeg",
    status: "uploading",
    progress: 40,
    attempts: 0,
    error: null,
    errorCode: null,
    photoId: "photo-1",
    uploadKind: "simple",
    fromRegistry: false,
    file: null,
    partSize: null,
    uploadedParts: new Set(),
    partETags: new Map(),
    ...overrides,
  }
}

function setup(items: UploadItem[]) {
  const handlers = {
    onRetry: vi.fn(),
    onCancel: vi.fn(),
    onDiscard: vi.fn(),
  }
  render(<UploadQueueList items={items} {...handlers} />)
  return handlers
}

describe("UploadQueueList", () => {
  it("renders nothing when empty", () => {
    const { container } = render(
      <UploadQueueList
        items={[]}
        onRetry={vi.fn()}
        onCancel={vi.fn()}
        onDiscard={vi.fn()}
      />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it("hides rows once their photo is ready", () => {
    setup([
      item({ status: "ready", progress: 100 }),
      item({ key: "k2", filename: "b.jpg", status: "uploading" }),
    ])

    expect(screen.queryByText("a.jpg")).toBeNull()
    expect(screen.getByText("b.jpg")).toBeInTheDocument()
  })

  it("renders nothing when every item is ready", () => {
    const { container } = render(
      <UploadQueueList
        items={[item({ status: "ready", progress: 100 })]}
        onRetry={vi.fn()}
        onCancel={vi.fn()}
        onDiscard={vi.fn()}
      />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it("tracks the batch progress by photos the worker marked ready", () => {
    setup([
      item({ status: "ready", progress: 100 }),
      item({ key: "k2", filename: "b.jpg", status: "uploading", progress: 50 }),
    ])

    const batch = within(screen.getByTestId("upload-batch-progress"))
    expect(batch.getByText("1 of 2 ready")).toBeInTheDocument()
    expect(batch.getByText("50%")).toBeInTheDocument()
  })

  it("waits for READY before counting a photo in the batch progress", () => {
    setup([
      item({ status: "processing", progress: 100 }),
      item({ key: "k2", filename: "b.jpg", status: "uploading", progress: 100 }),
    ])

    const batch = within(screen.getByTestId("upload-batch-progress"))
    expect(batch.getByText("0 of 2 ready")).toBeInTheDocument()
    expect(batch.getByText("0%")).toBeInTheDocument()
  })

  it("hides the batch progress for a single file", () => {
    setup([item({ status: "uploading", progress: 10 })])

    expect(screen.queryByTestId("upload-batch-progress")).toBeNull()
  })

  it("shows progress and a cancel action while uploading", async () => {
    const handlers = setup([item()])
    expect(screen.getByText("a.jpg")).toBeInTheDocument()
    expect(screen.getByText(/40%/)).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /Cancel a.jpg/ }))
    expect(handlers.onCancel).toHaveBeenCalledWith("k1")
  })

  it("offers retry and remove for a failed upload with a local file", async () => {
    const handlers = setup([
      item({
        status: "failed",
        error: "Network error",
        file: new File([new Uint8Array(1)], "a.jpg", { type: "image/jpeg" }),
      }),
    ])
    expect(screen.getByRole("alert")).toHaveTextContent("Network error")

    await userEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(handlers.onRetry).toHaveBeenCalledWith("k1")

    await userEvent.click(screen.getByRole("button", { name: "Remove" }))
    expect(handlers.onDiscard).toHaveBeenCalledWith("k1")
  })

  it("links plan-limit failures to billing", () => {
    setup([
      item({
        status: "failed",
        error: "You have reached your plan's limit.",
        errorCode: "PLAN_LIMIT_REACHED",
        file: new File([new Uint8Array(1)], "a.jpg", { type: "image/jpeg" }),
      }),
    ])

    expect(screen.getByRole("link", { name: "View plans" })).toHaveAttribute(
      "href",
      "/billing"
    )
  })

  it("offers only remove for an interrupted registry entry", () => {
    setup([item({ status: "interrupted", file: null, fromRegistry: true })])
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull()
    expect(screen.getByRole("button", { name: "Remove" })).toBeInTheDocument()
  })
})
