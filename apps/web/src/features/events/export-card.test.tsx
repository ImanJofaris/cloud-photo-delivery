import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError, type components } from "@workspace/api-client"

vi.mock("./api", () => ({
  useEventExport: vi.fn(),
  useCreateEventExport: vi.fn(),
}))

import { useCreateEventExport, useEventExport } from "./api"
import { ExportCard } from "./export-card"

type Export = components["schemas"]["Export"]

const EXPORT_ID = "22222222-2222-2222-2222-222222222222"

function exportJob(overrides: Partial<Export> = {}): Export {
  return {
    id: EXPORT_ID,
    eventId: "event-1",
    status: "ready",
    fileSize: 1024 * 1024,
    expiresAt: new Date(Date.now() + 3_600_000).toISOString(),
    createdAt: "2026-09-16T12:00:00Z",
    downloadUrl: "https://r2.example/signed.zip",
    ...overrides,
  }
}

function mockExportQuery(overrides: Record<string, unknown> = {}) {
  vi.mocked(useEventExport).mockReturnValue({
    data: undefined,
    isPending: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  } as unknown as ReturnType<typeof useEventExport>)
}

describe("ExportCard", () => {
  const mutateAsync = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    window.sessionStorage.clear()
    mutateAsync.mockResolvedValue(
      exportJob({ status: "pending", fileSize: null, downloadUrl: null })
    )
    vi.mocked(useCreateEventExport).mockReturnValue({
      mutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useCreateEventExport>)
    mockExportQuery()
  })

  it("disables the action when the event has no photos", () => {
    render(<ExportCard eventId="event-1" photoCount={0} />)

    expect(
      screen.getByRole("button", { name: /export all photos/i })
    ).toBeDisabled()
    expect(
      screen.getByText(/add photos before generating an export/i)
    ).toBeInTheDocument()
  })

  it("persists only the export id in sessionStorage on create", async () => {
    render(<ExportCard eventId="event-1" photoCount={5} />)

    await userEvent.click(
      screen.getByRole("button", { name: /export all photos/i })
    )

    expect(mutateAsync).toHaveBeenCalledTimes(1)
    expect(window.sessionStorage.getItem("cpd:export:event-1")).toBe(EXPORT_ID)
    await waitFor(() =>
      expect(useEventExport).toHaveBeenLastCalledWith("event-1", EXPORT_ID)
    )
  })

  it("resumes polling from the stored export id", () => {
    window.sessionStorage.setItem("cpd:export:event-1", "exp-stored")
    mockExportQuery({ data: exportJob({ id: "exp-stored" }) })

    render(<ExportCard eventId="event-1" photoCount={5} />)

    expect(useEventExport).toHaveBeenCalledWith("event-1", "exp-stored")
    expect(
      screen.getByRole("button", { name: /download zip/i })
    ).toHaveAttribute("href", "https://r2.example/signed.zip")
  })

  it("shows size and a download link when ready", () => {
    window.sessionStorage.setItem("cpd:export:event-1", EXPORT_ID)
    mockExportQuery({ data: exportJob() })

    render(<ExportCard eventId="event-1" photoCount={5} />)

    expect(screen.getByText(/ready/i)).toBeInTheDocument()
    expect(screen.getByText(/1\.0 MB/)).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: /download zip/i })
    ).toHaveAttribute("href", "https://r2.example/signed.zip")
  })

  it("offers a fresh export after a failure", () => {
    window.sessionStorage.setItem("cpd:export:event-1", EXPORT_ID)
    mockExportQuery({
      data: exportJob({ status: "failed", fileSize: null, downloadUrl: null }),
    })

    render(<ExportCard eventId="event-1" photoCount={5} />)

    expect(screen.getByText(/could not be generated/i)).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: /generate new export/i })
    ).toBeInTheDocument()
  })

  it("offers a fresh export after expiry", () => {
    window.sessionStorage.setItem("cpd:export:event-1", EXPORT_ID)
    mockExportQuery({
      data: exportJob({ status: "expired", fileSize: null, downloadUrl: null }),
    })

    render(<ExportCard eventId="event-1" photoCount={5} />)

    expect(screen.getByText(/has expired/i)).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: /generate new export/i })
    ).toBeInTheDocument()
  })

  it("clears a stale export id the API no longer knows", async () => {
    window.sessionStorage.setItem("cpd:export:event-1", "exp-stale")
    mockExportQuery({
      isError: true,
      error: new ApiError("EXPORT_NOT_FOUND", "gone", 404),
    })

    render(<ExportCard eventId="event-1" photoCount={5} />)

    await waitFor(() =>
      expect(window.sessionStorage.getItem("cpd:export:event-1")).toBeNull()
    )
    expect(
      screen.getByRole("button", { name: /export all photos/i })
    ).toBeInTheDocument()
  })
})
