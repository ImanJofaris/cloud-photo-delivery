import { NextResponse, type NextRequest } from "next/server"

import { REFRESH_COOKIE } from "@/lib/auth/cookies"

const PROTECTED_PREFIX = "/dashboard"
const AUTH_PAGES = ["/login", "/signup"]

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl
  const hasSession = request.cookies.has(REFRESH_COOKIE)

  if (!hasSession && pathname.startsWith(PROTECTED_PREFIX)) {
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
  matcher: ["/dashboard/:path*", "/login", "/signup"],
}
