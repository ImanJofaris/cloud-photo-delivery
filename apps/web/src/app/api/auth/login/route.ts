import { loginHandler, makeGoFetch, serviceUnavailable } from "@/lib/auth/bff"
import { cookieSink } from "@/lib/auth/cookies"
import { serverApiBaseUrl } from "@/lib/env"

export async function POST(request: Request) {
  try {
    return await loginHandler(
      request,
      makeGoFetch(serverApiBaseUrl()),
      cookieSink
    )
  } catch {
    return serviceUnavailable()
  }
}
