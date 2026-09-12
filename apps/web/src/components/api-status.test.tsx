import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { ApiStatus } from "./api-status"

describe("ApiStatus", () => {
  it("renders the online state", () => {
    render(<ApiStatus status="online" />)
    expect(screen.getByTestId("api-status")).toHaveTextContent("API online")
  })

  it("renders the offline state", () => {
    render(<ApiStatus status="offline" />)
    expect(screen.getByTestId("api-status")).toHaveTextContent("API offline")
  })

  it("renders the unconfigured state", () => {
    render(<ApiStatus status="unconfigured" />)
    expect(screen.getByTestId("api-status")).toHaveTextContent(
      "API not configured"
    )
  })
})
