import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import * as React from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { resetApiClientForTests } from "@/lib/auth/api"
import { resetSessionForTests } from "@/lib/auth/session"

import { PhotoGrid, sortPhotos } from "./photo-grid"

type Photo = components["schemas"]["Photo"]
type PhotoList = components["schemas"]["PhotoList"]

function photo(id: string, overrides: Partial<Photo> = {}): Photo {
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
    ...overrides,
  }
}

function stubPhotos(photos: Photo[], nextCursor: string | null = null) {
  const calls: string[] = []
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.toString()
            : input.url
      calls.push(url)
      if (url.includes("/url")) {
        return Response.json({
          data: {
            url: "https://img.test/thumb.jpg",
            expiresAt: "2026-10-01T01:00:00Z",
          },
          error: null,
        })
      }
      const status = new URL(url).searchParams.get("status")
      const items = status
        ? photos.filter((candidate) => candidate.status === status)
        : photos
      const page: PhotoList = { items, nextCursor }
      return Response.json({ data: page, error: null })
    })
  )
  return { calls }
}

function renderGrid(
  props: Partial<React.ComponentProps<typeof PhotoGrid>> = {}
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const onDelete = vi.fn()
  render(
    <QueryClientProvider client={queryClient}>
      <PhotoGrid eventId="event-1" onDelete={onDelete} {...props} />
    </QueryClientProvider>
  )
  return { onDelete }
}

describe("sortPhotos", () => {
  it("orders by createdAt in both directions", () => {
    const older = photo("old", { createdAt: "2026-09-01T00:00:00Z" })
    const newer = photo("new", { createdAt: "2026-10-01T00:00:00Z" })

    expect(sortPhotos([older, newer], "newest").map((p) => p.id)).toEqual([
      "new",
      "old",
    ])
    expect(sortPhotos([newer, older], "oldest").map((p) => p.id)).toEqual([
      "old",
      "new",
    ])
  })
})

describe("PhotoGrid", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080"
    resetSessionForTests()
    resetApiClientForTests()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("shows the event photo count above the tiles", async () => {
    stubPhotos([photo("p1"), photo("p2")])
    renderGrid({ photoCount: 486 })

    expect(await screen.findByText("p1.jpg")).toBeInTheDocument()
    expect(screen.getByText("486 photos")).toBeInTheDocument()
  })

  it("filters by status through the API", async () => {
    const { calls } = stubPhotos([
      photo("p1"),
      photo("p2", { status: "PROCESSING" }),
    ])
    renderGrid()
    await screen.findByText("p1.jpg")

    await userEvent.click(screen.getByRole("button", { name: "Processing" }))

    await waitFor(() => {
      expect(calls.some((url) => url.includes("status=PROCESSING"))).toBe(true)
    })
    expect(await screen.findByText("p2.jpg")).toBeInTheDocument()
    expect(screen.queryByText("p1.jpg")).toBeNull()
    expect(screen.getByText("1 processing photo")).toBeInTheDocument()
  })

  it("shows an empty state when a filter matches nothing", async () => {
    stubPhotos([photo("p1")])
    renderGrid()
    await screen.findByText("p1.jpg")

    await userEvent.click(screen.getByRole("button", { name: "Failed" }))

    expect(await screen.findByText("No failed photos")).toBeInTheDocument()
  })

  it("deletes a single photo from the tile overlay", async () => {
    stubPhotos([photo("p1")])
    const { onDelete } = renderGrid()
    await screen.findByText("p1.jpg")

    await userEvent.click(screen.getByRole("button", { name: "Delete p1.jpg" }))

    expect(onDelete).toHaveBeenCalledWith([
      expect.objectContaining({ id: "p1" }),
    ])
  })

  it("selects photos and hands the batch to delete", async () => {
    stubPhotos([photo("p1"), photo("p2")])
    const { onDelete } = renderGrid()
    await screen.findByText("p1.jpg")

    await userEvent.click(screen.getByRole("button", { name: "Select" }))
    expect(screen.queryByRole("button", { name: "Delete p1.jpg" })).toBeNull()

    await userEvent.click(screen.getByRole("button", { name: "Select p1.jpg" }))
    await userEvent.click(screen.getByRole("button", { name: "Select p2.jpg" }))
    expect(screen.getByText("2 selected")).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: "Delete" }))

    expect(onDelete).toHaveBeenCalledWith([
      expect.objectContaining({ id: "p1" }),
      expect.objectContaining({ id: "p2" }),
    ])
  })

  it("clears the selection without leaving select mode", async () => {
    stubPhotos([photo("p1")])
    renderGrid()
    await screen.findByText("p1.jpg")

    await userEvent.click(screen.getByRole("button", { name: "Select" }))
    await userEvent.click(screen.getByRole("button", { name: "Select p1.jpg" }))
    expect(screen.getByText("1 selected")).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: "Clear" }))
    expect(screen.getByText("0 selected")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Select" })).toHaveAttribute(
      "aria-pressed",
      "true"
    )
  })

  it("marks the cover photo with a star", async () => {
    stubPhotos([photo("p1"), photo("p2")])
    renderGrid({ coverPhotoId: "p2" })
    await screen.findByText("p1.jpg")

    expect(
      screen.getByRole("img", { name: "Cover photo" })
    ).toBeInTheDocument()
  })

  it("shows the empty state before the first upload", async () => {
    stubPhotos([])
    renderGrid()

    expect(await screen.findByText("No photos yet")).toBeInTheDocument()
  })

  it("reports a load failure with a retry action", async () => {
    const fetchMock = vi.fn(async () =>
      Response.json(
        { data: null, error: { code: "INTERNAL", message: "boom" } },
        { status: 500 }
      )
    )
    vi.stubGlobal("fetch", fetchMock)
    renderGrid()

    expect(
      await screen.findByText("Could not load photos. Please try again.")
    ).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument()
  })
})
