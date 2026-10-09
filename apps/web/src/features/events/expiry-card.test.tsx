import { render, screen } from "@testing-library/react"
import * as React from "react"
import { describe, expect, it, vi } from "vitest"

vi.mock("./extend-expiry-dialog", () => ({
  ExtendExpiryDialog: ({ trigger }: { trigger: React.ReactElement }) =>
    trigger,
}))

import { ExpiryCard, expiryCardCopy } from "./expiry-card"

const DAY_MS = 86_400_000

function isoFromNow(days: number): string {
  return new Date(Date.now() + days * DAY_MS).toISOString()
}

describe("expiryCardCopy", () => {
  it("handles a gallery without an expiry", () => {
    expect(expiryCardCopy(null)).toEqual({
      tone: "neutral",
      title: "This gallery does not expire",
      description:
        "It stays online until you set an expiry or archive the event.",
    })
  })

  it("counts down to an upcoming expiry", () => {
    const copy = expiryCardCopy(isoFromNow(3))
    expect(copy.tone).toBe("warning")
    expect(copy.title).toMatch(/^Gallery closes .* · in 3 days$/)
  })

  it("labels a closed gallery", () => {
    const copy = expiryCardCopy(isoFromNow(-2))
    expect(copy.tone).toBe("expired")
    expect(copy.title).toMatch(/^Gallery closed /)
    expect(copy.description).toBe(
      "Guests see a closed page until you extend the event."
    )
  })
})

describe("ExpiryCard", () => {
  it("renders the countdown, progress, and extend action", () => {
    render(
      <ExpiryCard
        eventId="event-1"
        status="active"
        createdAt={isoFromNow(-7)}
        expiresAt={isoFromNow(3)}
      />
    )

    expect(screen.getByText(/in 3 days/)).toBeInTheDocument()
    expect(screen.getByRole("progressbar")).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "Extend expiry" })
    ).toBeInTheDocument()
  })

  it("omits the progress bar without a usable window", () => {
    render(
      <ExpiryCard
        eventId="event-1"
        status="active"
        createdAt="2026-09-01T00:00:00Z"
        expiresAt={null}
      />
    )

    expect(screen.getByText("This gallery does not expire")).toBeInTheDocument()
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument()
  })
})
