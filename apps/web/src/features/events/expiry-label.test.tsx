import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { ExpiryLabel } from "./expiry-label"

const now = Date.parse("2026-09-16T12:00:00Z")

describe("ExpiryLabel", () => {
  it("warns with a warning tone at exactly seven days", () => {
    render(<ExpiryLabel expiresAt="2026-09-23T12:00:00Z" now={now} />)

    expect(screen.getByText("Expires in 7 days")).toHaveClass("text-amber-600")
  })

  it("uses a destructive tone once expired", () => {
    render(<ExpiryLabel expiresAt="2026-09-14T12:00:00Z" now={now} />)

    expect(screen.getByText("Expired 2 days ago")).toHaveClass(
      "text-destructive"
    )
  })

  it("labels never-expiring events", () => {
    render(<ExpiryLabel expiresAt={null} now={now} />)

    expect(screen.getByText("Never expires")).toHaveClass(
      "text-muted-foreground"
    )
  })
})
