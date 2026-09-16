import { NextRequest } from "next/server"
import { describe, expect, it } from "vitest"

import { REFRESH_COOKIE } from "@/lib/auth/cookies"

import { PROTECTED_PREFIXES, isProtectedPath, proxy } from "./proxy"

function request(path: string, signedIn = false): NextRequest {
  const headers = new Headers()
  if (signedIn) {
    headers.set("cookie", `${REFRESH_COOKIE}=refresh-token`)
  }
  return new NextRequest(
    new Request(`http://localhost:3000${path}`, { headers })
  )
}

describe("isProtectedPath", () => {
  it("protects every dashboard prefix and its nested routes", () => {
    expect(PROTECTED_PREFIXES).toEqual([
      "/dashboard",
      "/analytics",
      "/events",
      "/account",
      "/devices",
      "/branding",
      "/billing",
    ])
    for (const prefix of PROTECTED_PREFIXES) {
      expect(isProtectedPath(prefix)).toBe(true)
      expect(isProtectedPath(`${prefix}/nested`)).toBe(true)
    }
  })

  it("does not match lookalike paths", () => {
    expect(isProtectedPath("/billing-history")).toBe(false)
    expect(isProtectedPath("/event")).toBe(false)
    expect(isProtectedPath("/e/wedding")).toBe(false)
  })
})

describe("proxy", () => {
  it("redirects each protected prefix to login when the cookie is absent", () => {
    for (const path of [
      "/dashboard",
      "/analytics",
      "/events",
      "/events/123",
      "/account",
      "/devices",
      "/branding",
      "/billing",
    ]) {
      const response = proxy(request(path))
      expect(response.status).toBe(307)
      const location = new URL(response.headers.get("location") ?? "")
      expect(location.pathname).toBe("/login")
      expect(location.searchParams.get("next")).toBe(path)
    }
  })

  it("lets authenticated users through to protected paths", () => {
    const response = proxy(request("/billing", true))
    expect(response.headers.get("location")).toBeNull()
  })

  it("sends authenticated users from auth pages to the dashboard", () => {
    const response = proxy(request("/login", true))
    const location = new URL(response.headers.get("location") ?? "")
    expect(location.pathname).toBe("/dashboard")
  })

  it("leaves public gallery pages alone", () => {
    const response = proxy(request("/e/wedding"))
    expect(response.headers.get("location")).toBeNull()
  })
})
