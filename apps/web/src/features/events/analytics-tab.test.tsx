import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

vi.mock("@/features/analytics/api", () => ({
  useEventAnalytics: vi.fn(),
}))

import { useEventAnalytics } from "@/features/analytics/api"

import { EventAnalyticsTab } from "./analytics-tab"

describe("EventAnalyticsTab", () => {
  it("queries analytics for the event with the default window", () => {
    vi.mocked(useEventAnalytics).mockReturnValue({
      data: undefined,
      isPending: true,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useEventAnalytics>)

    render(<EventAnalyticsTab eventId="event-1" />)

    expect(useEventAnalytics).toHaveBeenCalledWith("event-1", 30)
  })

  it("renders event totals and the chart", () => {
    vi.mocked(useEventAnalytics).mockReturnValue({
      data: {
        eventId: "event-1",
        photoCount: 12,
        totals: {
          galleryViews: 20,
          uniqueVisitors: 8,
          downloads: 3,
          qrScans: 5,
        },
        daily: [
          {
            date: new Date().toISOString().slice(0, 10),
            galleryViews: 20,
            uniqueVisitors: 8,
            downloads: 3,
            qrScans: 5,
          },
        ],
      },
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useEventAnalytics>)

    render(<EventAnalyticsTab eventId="event-1" />)

    expect(screen.getByText("Gallery views")).toBeInTheDocument()
    expect(screen.getByText("20")).toBeInTheDocument()
    expect(screen.getByText("Photos (all time)")).toBeInTheDocument()
    expect(screen.getByText("12")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Views" })).toBeInTheDocument()
  })
})
