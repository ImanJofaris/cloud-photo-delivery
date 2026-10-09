import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import type { components } from "@workspace/api-client"

import { ViewsDownloadsCard } from "./views-downloads-card"

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

describe("ViewsDownloadsCard", () => {
  it("renders a legend and an accessible summary for both series", () => {
    render(
      <ViewsDownloadsCard
        daily={[recentDay(2, { galleryViews: 20, downloads: 3 })]}
        days={7}
      />
    )

    expect(screen.getByText("Views")).toBeInTheDocument()
    expect(screen.getByText("Downloads")).toBeInTheDocument()
    expect(
      screen.getByText(
        "Views: 20 total, downloads: 3 total over the last 7 UTC days."
      )
    ).toBeInTheDocument()
  })

  it("shows an empty state when the window has no activity", () => {
    render(<ViewsDownloadsCard days={30} daily={[]} />)

    expect(screen.getByText("No activity yet")).toBeInTheDocument()
    expect(screen.queryByText(/total over the last/)).not.toBeInTheDocument()
  })

  it("shows a skeleton while loading", () => {
    const { container } = render(
      <ViewsDownloadsCard days={7} loading daily={[]} />
    )

    expect(container.querySelector('[data-slot="skeleton"]')).not.toBeNull()
  })
})
