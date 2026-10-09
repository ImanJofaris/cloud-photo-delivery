import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

const DAY_MS = 86_400_000

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>()
  return {
    ...actual,
    useEventDashboard: vi.fn(),
    useEventPublicUrl: vi.fn(),
    useTransitionEvent: vi.fn(),
    useExtendEvent: vi.fn(),
    useEventExport: vi.fn(),
    useCreateEventExport: vi.fn(),
  }
})

vi.mock("@/features/analytics/api", () => ({
  useEventAnalytics: vi.fn(),
}))

vi.mock("@/features/photos/api", () => ({
  useEventPhotos: vi.fn(),
  usePhotoThumbnail: vi.fn(),
}))

vi.mock("@/lib/auth/api", () => ({
  apiBlobCall: vi.fn(),
}))

import { useEventAnalytics } from "@/features/analytics/api"
import { apiBlobCall } from "@/lib/auth/api"
import { useEventPhotos, usePhotoThumbnail } from "@/features/photos/api"

import {
  useCreateEventExport,
  useEventDashboard,
  useEventExport,
  useEventPublicUrl,
  useExtendEvent,
  useTransitionEvent,
} from "./api"
import { EventOverview } from "./event-overview"

type Event = components["schemas"]["Event"]
type Photo = components["schemas"]["Photo"]

const event: Event = {
  id: "event-1",
  name: "Aina & Haziq Wedding",
  slug: "aina-haziq-wedding",
  clientName: "Aina",
  location: "Dewan Perdana Felda, KL",
  eventDate: "2026-10-04",
  status: "active",
  storageBytes: 0,
  photoCount: 486,
  guestCount: 214,
  expiresAt: new Date(Date.now() + 3 * DAY_MS).toISOString(),
  createdAt: new Date(Date.now() - 10 * DAY_MS).toISOString(),
  updatedAt: new Date(Date.now() - DAY_MS).toISOString(),
}

const photo: Photo = {
  id: "photo-1",
  filename: "IMG_4021.jpg",
  mimeType: "image/jpeg",
  fileSize: 1024,
  width: 800,
  height: 600,
  status: "READY",
  variants: ["thumbnail"],
  errorMessage: null,
  createdAt: new Date(Date.now() - 60_000).toISOString(),
}

describe("EventOverview", () => {
  const onShowQr = vi.fn()
  const onViewPhotos = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    URL.createObjectURL = vi.fn(() => "blob:qr")
    URL.revokeObjectURL = vi.fn()

    vi.mocked(useEventDashboard).mockReturnValue({
      data: { photoCount: 486, storageBytes: 1024, guestCount: 214 },
      isPending: false,
    } as unknown as ReturnType<typeof useEventDashboard>)
    vi.mocked(useEventPublicUrl).mockReturnValue({
      data: { url: "https://public.test/e/aina-haziq-wedding" },
    } as unknown as ReturnType<typeof useEventPublicUrl>)
    vi.mocked(useTransitionEvent).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useTransitionEvent>)
    vi.mocked(useExtendEvent).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useExtendEvent>)
    vi.mocked(useEventExport).mockReturnValue({
      data: undefined,
      isPending: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useEventExport>)
    vi.mocked(useCreateEventExport).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useCreateEventExport>)
    vi.mocked(useEventAnalytics).mockReturnValue({
      data: {
        eventId: "event-1",
        photoCount: 486,
        totals: {
          galleryViews: 1022,
          uniqueVisitors: 214,
          downloads: 128,
          qrScans: 312,
        },
        daily: [],
      },
      isPending: false,
    } as unknown as ReturnType<typeof useEventAnalytics>)
    vi.mocked(useEventPhotos).mockReturnValue({
      data: { pages: [{ items: [photo], nextCursor: null }] },
      isPending: false,
      isError: false,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: vi.fn(),
    } as unknown as ReturnType<typeof useEventPhotos>)
    vi.mocked(usePhotoThumbnail).mockReturnValue({
      data: undefined,
      isPending: false,
    } as unknown as ReturnType<typeof usePhotoThumbnail>)
    vi.mocked(apiBlobCall)
      .mockResolvedValueOnce(new Blob(["png"], { type: "image/png" }))
      .mockResolvedValueOnce(
        new Blob(["<svg><rect /></svg>"], { type: "image/svg+xml" })
      )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("shows the expiry countdown, QR scans, stats, and latest uploads", async () => {
    render(
      <EventOverview
        event={event}
        onShowQr={onShowQr}
        onViewPhotos={onViewPhotos}
      />
    )

    expect(
      screen.getByText(/^Gallery closes .* · in 3 days$/)
    ).toBeInTheDocument()
    expect(screen.getByText("Gallery QR")).toBeInTheDocument()
    expect(
      await screen.findByAltText("QR code for Aina & Haziq Wedding")
    ).toBeInTheDocument()
    expect(await screen.findByText("312 scans")).toBeInTheDocument()

    expect(screen.getByText("486")).toBeInTheDocument()
    expect(screen.getByText("214")).toBeInTheDocument()
    expect(screen.getByText("128")).toBeInTheDocument()
    expect(screen.getByText("1,022")).toBeInTheDocument()

    expect(screen.getByText("Latest uploads")).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "View all photos (IMG_4021.jpg)" })
    ).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /View all 486/ }))
    expect(onViewPhotos).toHaveBeenCalled()
  })

  it("keeps the share, details, status, and export flows on the overview", async () => {
    render(
      <EventOverview
        event={event}
        onShowQr={onShowQr}
        onViewPhotos={onViewPhotos}
      />
    )

    expect(screen.getByText("Share")).toBeInTheDocument()
    expect(screen.getByText("Details")).toBeInTheDocument()
    expect(screen.getByText("QR scans")).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Mark completed" })
    ).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: /export all photos/i })
    ).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /^QR code$/ }))
    expect(onShowQr).toHaveBeenCalled()
  })
})
