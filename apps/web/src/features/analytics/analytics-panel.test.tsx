import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError, type components } from "@workspace/api-client"

vi.mock("./api", () => ({
  useAccountAnalytics: vi.fn(),
}))

vi.mock("@/features/billing/api", () => ({
  useSubscription: vi.fn(),
}))

import { useSubscription } from "@/features/billing/api"

import { useAccountAnalytics } from "./api"
import { AnalyticsPanel } from "./analytics-panel"
import { formatUtcWeekday, formatUtcWindowRange } from "./format"

type AccountAnalytics = components["schemas"]["AccountAnalytics"]
type AnalyticsDay = components["schemas"]["AnalyticsDay"]

const DAY_MS = 86_400_000

function recentDay(
  offset: number,
  overrides: Partial<AnalyticsDay> = {}
): AnalyticsDay {
  return {
    date: new Date(Date.now() - offset * DAY_MS).toISOString().slice(0, 10),
    galleryViews: 0,
    uniqueVisitors: 0,
    downloads: 0,
    qrScans: 0,
    ...overrides,
  }
}

const busiest = recentDay(2, { galleryViews: 486, downloads: 62 })
const runnerUp = recentDay(3, { galleryViews: 412, downloads: 41 })

const analytics: AccountAnalytics = {
  eventCount: 47,
  photoCount: 12486,
  totals: {
    galleryViews: 8214,
    uniqueVisitors: 3108,
    downloads: 1204,
    qrScans: 2886,
  },
  daily: [busiest, runnerUp],
}

function mockQuery(overrides: Record<string, unknown> = {}) {
  vi.mocked(useAccountAnalytics).mockReturnValue({
    data: analytics,
    isPending: false,
    isError: false,
    error: null,
    ...overrides,
  } as unknown as ReturnType<typeof useAccountAnalytics>)
}

function mockSubscription(overrides: Record<string, unknown> = {}) {
  vi.mocked(useSubscription).mockReturnValue({
    data: {
      usage: { events: 3, storageBytes: 96 * 1024 ** 3 },
    },
    isPending: false,
    isError: false,
    error: null,
    ...overrides,
  } as unknown as ReturnType<typeof useSubscription>)
}

describe("AnalyticsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockQuery()
    mockSubscription()
  })

  it("defaults to the 30-day window and renders counters with captions", () => {
    render(<AnalyticsPanel />)

    expect(useAccountAnalytics).toHaveBeenCalledWith(30)
    expect(screen.getByText("Gallery views")).toBeInTheDocument()
    expect(screen.getByText("8,214")).toBeInTheDocument()
    expect(screen.getByText("3,108")).toBeInTheDocument()
    expect(screen.getByText("1,204")).toBeInTheDocument()
    expect(screen.getByText("2,886")).toBeInTheDocument()
    expect(screen.getByText(formatUtcWindowRange(30))).toBeInTheDocument()
    expect(screen.getByText("sum of daily uniques")).toBeInTheDocument()
  })

  it("switches the window with the segmented control", async () => {
    render(<AnalyticsPanel />)

    await userEvent.click(screen.getByRole("tab", { name: "7 days" }))

    expect(useAccountAnalytics).toHaveBeenLastCalledWith(7)
  })

  it("shows placeholders while loading instead of zeros", () => {
    mockQuery({ data: undefined, isPending: true })
    mockSubscription({ data: undefined, isPending: true })

    render(<AnalyticsPanel />)

    expect(screen.getAllByText("—")).toHaveLength(7)
    expect(screen.queryByText("0")).not.toBeInTheDocument()
  })

  it("maps errors without leaking server messages", () => {
    mockQuery({
      data: undefined,
      isError: true,
      error: new ApiError("VALIDATION_ERROR", "raw server detail", 422),
    })

    render(<AnalyticsPanel />)

    expect(
      screen.getByText(/that analytics window is not valid/i)
    ).toBeInTheDocument()
    expect(screen.queryByText(/raw server detail/)).not.toBeInTheDocument()
  })

  it("lists the busiest days from the daily series", () => {
    render(<AnalyticsPanel />)

    expect(screen.getByText(formatUtcWeekday(busiest.date))).toBeInTheDocument()
    expect(screen.getByText("486 views, 62 downloads")).toBeInTheDocument()
    expect(screen.getByText("412 views, 41 downloads")).toBeInTheDocument()
  })

  it("shows an empty state when the window has no activity", () => {
    mockQuery({ data: { ...analytics, daily: [] } })

    render(<AnalyticsPanel />)

    expect(screen.getByText("No activity in this window yet.")).toBeInTheDocument()
  })

  it("shows all-time events, photos, and storage", () => {
    render(<AnalyticsPanel />)

    expect(screen.getByText("All time")).toBeInTheDocument()
    expect(screen.getByText("47")).toBeInTheDocument()
    expect(screen.getByText("12,486")).toBeInTheDocument()
    expect(screen.getByText("96 GB")).toBeInTheDocument()
  })

  it("falls back to a placeholder when storage is unavailable", () => {
    mockSubscription({ data: undefined, isPending: false, isError: true })

    render(<AnalyticsPanel />)

    expect(screen.getByText("Storage")).toBeInTheDocument()
    expect(screen.getAllByText("—")).toHaveLength(1)
  })

  it("keeps the UTC footnote", () => {
    render(<AnalyticsPanel />)

    expect(
      screen.getByText(/Counters are recorded per UTC day/)
    ).toBeInTheDocument()
  })
})
