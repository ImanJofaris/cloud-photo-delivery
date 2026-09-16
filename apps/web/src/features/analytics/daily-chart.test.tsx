import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it } from "vitest"

import type { components } from "@workspace/api-client"

import { DailyChart } from "./daily-chart"

type AnalyticsDay = components["schemas"]["AnalyticsDay"]

const DAY_MS = 86_400_000

function recentDay(offset: number, overrides: Partial<AnalyticsDay> = {}) {
  return {
    date: new Date(Date.now() - offset * DAY_MS).toISOString().slice(0, 10),
    galleryViews: 0,
    uniqueVisitors: 0,
    downloads: 0,
    qrScans: 0,
    ...overrides,
  }
}

describe("DailyChart", () => {
  it("zero-fills the window and summarises the active metric", () => {
    render(
      <DailyChart
        daily={[recentDay(2, { galleryViews: 2, downloads: 1 })]}
        days={7}
      />
    )

    expect(
      screen.getByText("Views: 2 total over the last 7 UTC days.")
    ).toBeInTheDocument()
  })

  it("switches metrics one at a time", async () => {
    render(
      <DailyChart
        daily={[recentDay(1, { galleryViews: 5, qrScans: 3 })]}
        days={7}
      />
    )

    expect(screen.getByRole("button", { name: "Views" })).toHaveAttribute(
      "aria-pressed",
      "true"
    )

    await userEvent.click(screen.getByRole("button", { name: "QR scans" }))

    expect(screen.getByRole("button", { name: "QR scans" })).toHaveAttribute(
      "aria-pressed",
      "true"
    )
    expect(screen.getByRole("button", { name: "Views" })).toHaveAttribute(
      "aria-pressed",
      "false"
    )
    expect(
      screen.getByText("QR scans: 3 total over the last 7 UTC days.")
    ).toBeInTheDocument()
  })

  it("shows an empty state when the window has no activity", () => {
    render(<DailyChart daily={[]} days={30} />)

    expect(screen.getByText("No activity yet")).toBeInTheDocument()
    expect(
      screen.queryByRole("button", { name: "Views" })
    ).not.toBeInTheDocument()
  })
})
