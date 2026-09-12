import { cookies } from "next/headers"

import type { CookieSink } from "./bff"

export const REFRESH_COOKIE = "cpd_refresh"
export const REFRESH_COOKIE_MAX_AGE = 60 * 60 * 24 * 30

export function refreshCookieOptions(maxAge = REFRESH_COOKIE_MAX_AGE) {
  return {
    httpOnly: true,
    sameSite: "lax" as const,
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge,
  }
}

export const cookieSink: CookieSink = {
  async read() {
    const store = await cookies()
    return store.get(REFRESH_COOKIE)?.value ?? null
  },
  async write(token) {
    const store = await cookies()
    store.set(REFRESH_COOKIE, token, refreshCookieOptions())
  },
  async clear() {
    const store = await cookies()
    store.set(REFRESH_COOKIE, "", refreshCookieOptions(0))
  },
}
