import { NextResponse, type NextRequest } from "next/server"

import { REFRESH_COOKIE } from "@/lib/auth/cookies"

export const PROTECTED_PREFIXES = [
  "/dashboard",
  "/analytics",
  "/events",
  "/account",
  "/devices",
  "/branding",
  "/billing",
  "/admin",
]

const AUTH_PAGES = ["/login", "/signup"]

export function isProtectedPath(pathname: string): boolean {
  return PROTECTED_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`)
  )
}

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl
  const hasSession = request.cookies.has(REFRESH_COOKIE)

  if (!hasSession && isProtectedPath(pathname)) {
    const url = request.nextUrl.clone()
    url.pathname = "/login"
    url.search = ""
    url.searchParams.set("next", pathname)
    return NextResponse.redirect(url)
  }

  if (hasSession && AUTH_PAGES.includes(pathname)) {
    const url = request.nextUrl.clone()
    url.pathname = "/dashboard"
    url.search = ""
    return NextResponse.redirect(url)
  }

  return NextResponse.next()
}

export const config = {
  matcher: [
    "/dashboard/:path*",
    "/analytics/:path*",
    "/events/:path*",
    "/account/:path*",
    "/devices/:path*",
    "/branding/:path*",
    "/billing/:path*",
    "/admin/:path*",
    "/login",
    "/signup",
  ],
}
