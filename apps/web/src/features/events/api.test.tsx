import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook } from "@testing-library/react"
import * as React from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import { resetApiClientForTests } from "@/lib/auth/api"
import { resetSessionForTests } from "@/lib/auth/session"

import {
  exportRefetchInterval,
  EXPORT_POLL_INTERVAL_MS,
  useCreateEventExport,
  useEventExport,
  useExtendEvent,
} from "./api"
import { eventKeys } from "./keys"

type Event = components["schemas"]["Event"]
type Export = components["schemas"]["Export"]

function eventFixture(overrides: Partial<Event> = {}): Event {
  return {
    id: "event-1",
    name: "Wedding",
    slug: "wedding",
    eventDate: "2026-09-01",
    status: "expired",
    storageBytes: 0,
    photoCount: 0,
    guestCount: 0,
    expiresAt: "2026-09-01T00:00:00Z",
    createdAt: "2026-09-01T00:00:00Z",
    updatedAt: "2026-09-01T00:00:00Z",
    ...overrides,
  }
}

function exportFixture(overrides: Partial<Export> = {}): Export {
  return {
    id: "exp-1",
    eventId: "event-1",
    status: "pending",
    fileSize: null,
    expiresAt: null,
    createdAt: "2026-09-16T12:00:00Z",
    downloadUrl: null,
    ...overrides,
  }
}

function wrapperFor(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
  }
}

describe("exportRefetchInterval", () => {
  it("polls while the export is pending or processing", () => {
    expect(exportRefetchInterval("pending")).toBe(EXPORT_POLL_INTERVAL_MS)
    expect(exportRefetchInterval("processing")).toBe(EXPORT_POLL_INTERVAL_MS)
  })

  it("stops on every terminal status", () => {
    expect(exportRefetchInterval("ready")).toBe(false)
    expect(exportRefetchInterval("failed")).toBe(false)
    expect(exportRefetchInterval("expired")).toBe(false)
    expect(exportRefetchInterval(undefined)).toBe(false)
  })
})

describe("event export and extend hooks", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080"
    resetSessionForTests()
    resetApiClientForTests()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("does not poll without an export id", () => {
    const fetchMock = vi.fn()
    vi.stubGlobal("fetch", fetchMock)
    const queryClient = new QueryClient()

    renderHook(() => useEventExport("event-1", null), {
      wrapper: wrapperFor(queryClient),
    })

    expect(fetchMock).not.toHaveBeenCalled()
  })

  it("caches a created export under its own key", async () => {
    const created = exportFixture()
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ data: created, error: null }))
    )
    const queryClient = new QueryClient()

    const { result } = renderHook(() => useCreateEventExport("event-1"), {
      wrapper: wrapperFor(queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync()
    })

    expect(
      queryClient.getQueryData(eventKeys.export("event-1", "exp-1"))
    ).toEqual(created)
  })

  it("extends an event, seeds the detail cache, and invalidates lists", async () => {
    const extended = eventFixture({
      status: "active",
      expiresAt: "2026-12-16T12:00:00Z",
    })
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ data: extended, error: null }))
    )
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    queryClient.setQueryData(eventKeys.detail("event-1"), eventFixture())
    const invalidate = vi.spyOn(queryClient, "invalidateQueries")

    const { result } = renderHook(() => useExtendEvent("event-1"), {
      wrapper: wrapperFor(queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync(30)
    })

    expect(queryClient.getQueryData(eventKeys.detail("event-1"))).toEqual(
      extended
    )
    expect(invalidate).toHaveBeenCalledWith({ queryKey: eventKeys.lists() })
  })
})
