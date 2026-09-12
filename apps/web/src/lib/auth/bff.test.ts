import { describe, expect, it, vi } from "vitest"

import {
  loginHandler,
  logoutHandler,
  makeGoFetch,
  refreshHandler,
  signupHandler,
  type CookieSink,
} from "./bff"

function jsonResponse(body: unknown, status = 200) {
  return Response.json(body, { status })
}

function memoryCookie(initial: string | null = null): CookieSink & {
  value: string | null
  writes: number
  clears: number
} {
  const sink = {
    value: initial,
    writes: 0,
    clears: 0,
    read() {
      return sink.value
    },
    write(token: string) {
      sink.value = token
      sink.writes += 1
    },
    clear() {
      sink.value = null
      sink.clears += 1
    },
  }
  return sink
}

function request(body: unknown) {
  return new Request("http://web.test/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })
}

const successPayload = {
  data: {
    accessToken: "access_1",
    refreshToken: "refresh_1",
    expiresIn: 900,
    user: { id: "u1", email: "a@b.com", businessName: "Iman Booth" },
  },
  error: null,
}

describe("loginHandler / signupHandler", () => {
  it("stores the refresh cookie and strips the refresh token from the response", async () => {
    const cookie = memoryCookie()
    const goFetch = vi.fn(async () => jsonResponse(successPayload))

    const response = await loginHandler(
      request({ email: "a@b.com", password: "secret" }),
      goFetch,
      cookie
    )

    expect(response.status).toBe(200)
    const payload = (await response.json()) as {
      data: Record<string, unknown>
    }
    expect(payload.data.accessToken).toBe("access_1")
    expect(payload.data.refreshToken).toBeUndefined()
    expect(cookie.value).toBe("refresh_1")

    const [path] = goFetch.mock.calls[0] as unknown as [string]
    expect(path).toBe("/api/v1/auth/login")
  })

  it("passes Go errors through without setting a cookie", async () => {
    const cookie = memoryCookie()
    const goFetch = vi.fn(async () =>
      jsonResponse(
        {
          data: null,
          error: {
            code: "INVALID_CREDENTIALS",
            message: "Invalid credentials",
          },
        },
        401
      )
    )

    const response = await loginHandler(
      request({ email: "a@b.com", password: "wrong" }),
      goFetch,
      cookie
    )

    expect(response.status).toBe(401)
    expect(cookie.value).toBeNull()
    expect(cookie.writes).toBe(0)
  })

  it("rejects invalid JSON with 422", async () => {
    const cookie = memoryCookie()
    const goFetch = vi.fn()
    const badRequest = new Request("http://web.test/api/auth/login", {
      method: "POST",
      body: "{not json",
    })

    const response = await loginHandler(badRequest, goFetch, cookie)

    expect(response.status).toBe(422)
    expect(goFetch).not.toHaveBeenCalled()
  })

  it("treats a malformed Go auth response as 502", async () => {
    const cookie = memoryCookie()
    const goFetch = vi.fn(async () =>
      jsonResponse({ data: { accessToken: "access_1" }, error: null })
    )

    const response = await signupHandler(
      request({ email: "a@b.com", password: "secret123" }),
      goFetch,
      cookie
    )

    expect(response.status).toBe(502)
    expect(cookie.writes).toBe(0)
  })
})

describe("refreshHandler", () => {
  it("returns 401 when no cookie is present", async () => {
    const cookie = memoryCookie()
    const goFetch = vi.fn()

    const response = await refreshHandler(request({}), goFetch, cookie)

    expect(response.status).toBe(401)
    expect(goFetch).not.toHaveBeenCalled()
  })

  it("rotates the cookie and strips the refresh token", async () => {
    const cookie = memoryCookie("refresh_old")
    const goFetch = vi.fn(async () =>
      jsonResponse({
        data: { ...successPayload.data, refreshToken: "refresh_new" },
        error: null,
      })
    )

    const response = await refreshHandler(request({}), goFetch, cookie)

    expect(response.status).toBe(200)
    const payload = (await response.json()) as {
      data: Record<string, unknown>
    }
    expect(payload.data.refreshToken).toBeUndefined()
    expect(cookie.value).toBe("refresh_new")
  })

  it("clears the cookie when the Go API rejects the rotation", async () => {
    const cookie = memoryCookie("refresh_old")
    const goFetch = vi.fn(async () =>
      jsonResponse(
        { data: null, error: { code: "UNAUTHENTICATED", message: "expired" } },
        401
      )
    )

    const response = await refreshHandler(request({}), goFetch, cookie)

    expect(response.status).toBe(401)
    expect(cookie.value).toBeNull()
    expect(cookie.clears).toBe(1)
  })

  it("coalesces concurrent refreshes for the same token", async () => {
    const cookie = memoryCookie("refresh_old")
    let release: () => void = () => {}
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const goFetch = vi.fn(async () => {
      await gate
      return jsonResponse({
        data: { ...successPayload.data, refreshToken: "refresh_new" },
        error: null,
      })
    })

    const first = refreshHandler(request({}), goFetch, cookie)
    const second = refreshHandler(request({}), goFetch, cookie)
    release()

    const responses = await Promise.all([first, second])

    expect(responses.every((response) => response.status === 200)).toBe(true)
    expect(goFetch).toHaveBeenCalledTimes(1)
  })
})

describe("logoutHandler", () => {
  it("revokes server-side and clears the cookie", async () => {
    const cookie = memoryCookie("refresh_old")
    const goFetch = vi.fn(async () => jsonResponse({ data: null, error: null }))

    const response = await logoutHandler(request({}), goFetch, cookie)

    expect(response.status).toBe(200)
    expect(cookie.value).toBeNull()
    expect(goFetch).toHaveBeenCalledTimes(1)
  })

  it("still clears the cookie when the Go API is unreachable", async () => {
    const cookie = memoryCookie("refresh_old")
    const goFetch = vi.fn(async () => {
      throw new Error("network down")
    })

    const response = await logoutHandler(request({}), goFetch, cookie)

    expect(response.status).toBe(200)
    expect(cookie.value).toBeNull()
  })
})

describe("makeGoFetch", () => {
  it("joins the base URL and path", async () => {
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        jsonResponse({ ok: true })
    )
    const goFetch = makeGoFetch("http://api.test/", fetchMock as typeof fetch)

    await goFetch("/api/v1/auth/login")

    expect(fetchMock).toHaveBeenCalledWith(
      "http://api.test/api/v1/auth/login",
      undefined
    )
  })
})
