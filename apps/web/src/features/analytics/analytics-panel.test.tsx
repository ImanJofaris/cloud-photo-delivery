import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError, type components } from "@workspace/api-client"

vi.mock("./api", () => ({
  useAccountAnalytics: vi.fn(),
}))

import { useAccountAnalytics } from "./api"
import { AnalyticsPanel } from "./analytics-panel"

type AccountAnalytics = components["schemas"]["AccountAnalytics"]

const analytics: AccountAnalytics = {
  eventCount: 3,
  photoCount: 1200,
  totals: {
    galleryViews: 9,
    uniqueVisitors: 4,
    downloads: 2,
    qrScans: 1,
  },
  daily: [],
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

describe("AnalyticsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockQuery()
  })

  it("defaults to the 30-day window and renders totals", () => {
    render(<AnalyticsPanel />)

    expect(useAccountAnalytics).toHaveBeenCalledWith(30)
    expect(screen.getByText("Gallery views")).toBeInTheDocument()
    expect(screen.getByText("1,200")).toBeInTheDocument()
    expect(screen.getByText("Events (all time)")).toBeInTheDocument()
  })

  it("shows placeholders while loading instead of zeros", () => {
    mockQuery({ data: undefined, isPending: true })

    render(<AnalyticsPanel />)

    expect(screen.getAllByText("—")).toHaveLength(4)
    expect(screen.queryByText("0")).not.toBeInTheDocument()
  })

  it("refetches with the selected window", async () => {
    render(<AnalyticsPanel />)

    await userEvent.click(
      screen.getByRole("combobox", { name: "Analytics window" })
    )
    await userEvent.click(
      await screen.findByRole("option", { name: "Last 7 days" })
    )

    expect(useAccountAnalytics).toHaveBeenLastCalledWith(7)
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
})
