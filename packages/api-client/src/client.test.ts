import { describe, expect, it, vi } from "vitest"

import { ApiError, createApiClient, unwrapEnvelope } from "./client"

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

describe("unwrapEnvelope", () => {
  it("returns the data payload on success", () => {
    const data = unwrapEnvelope<{ id: string }>({
      data: { data: { id: "evt_1" }, error: null },
      response: jsonResponse({}),
    })

    expect(data).toEqual({ id: "evt_1" })
  })

  it("throws ApiError with code and status from the envelope", () => {
    expect.assertions(3)

    try {
      unwrapEnvelope({
        data: {
          data: null,
          error: { code: "EVENT_NOT_FOUND", message: "Event not found" },
        },
        response: jsonResponse({}, 404),
      })
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError)
      expect((error as ApiError).code).toBe("EVENT_NOT_FOUND")
      expect((error as ApiError).status).toBe(404)
    }
  })

  it("unwraps the envelope when openapi-fetch puts it in the error slot", () => {
    expect.assertions(3)

    try {
      unwrapEnvelope({
        error: {
          data: null,
          error: { code: "UNAUTHORIZED", message: "Incorrect password" },
        },
        response: jsonResponse({}, 401),
      })
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError)
      expect((error as ApiError).code).toBe("UNAUTHORIZED")
      expect((error as ApiError).status).toBe(401)
    }
  })

  it("throws ApiError when openapi-fetch reports a transport error", () => {
    expect.assertions(2)

    try {
      unwrapEnvelope({
        error: { code: "VALIDATION_ERROR", message: "Invalid input" },
        response: jsonResponse({}, 422),
      })
    } catch (error) {
      expect((error as ApiError).code).toBe("VALIDATION_ERROR")
      expect((error as ApiError).status).toBe(422)
    }
  })

  it("throws ApiError for a non-ok response without a body", () => {
    expect.assertions(1)

    try {
      unwrapEnvelope({ response: jsonResponse({}, 500) })
    } catch (error) {
      expect((error as ApiError).status).toBe(500)
    }
  })

  it("captures Retry-After on rate-limited responses", () => {
    expect.assertions(3)

    const response = new Response(JSON.stringify({}), {
      status: 429,
      headers: { "Content-Type": "application/json", "Retry-After": "60" },
    })

    try {
      unwrapEnvelope({
        data: {
          data: null,
          error: { code: "RATE_LIMITED", message: "Too many requests" },
        },
        response,
      })
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError)
      expect((error as ApiError).status).toBe(429)
      expect((error as ApiError).retryAfterSeconds).toBe(60)
    }
  })

  it("leaves retryAfterSeconds null when the header is absent", () => {
    expect.assertions(1)

    try {
      unwrapEnvelope({
        data: { data: null, error: { code: "RATE_LIMITED", message: "x" } },
        response: jsonResponse({}, 429),
      })
    } catch (error) {
      expect((error as ApiError).retryAfterSeconds).toBeNull()
    }
  })
})

describe("createApiClient", () => {
  it("injects the bearer token from the token provider", async () => {
    const fetchMock = vi.fn((_input: RequestInfo | URL, _init?: RequestInit) =>
      Promise.resolve(jsonResponse({ data: { ok: true }, error: null }))
    )
    vi.stubGlobal("fetch", fetchMock)

    try {
      const client = createApiClient({
        baseUrl: "http://api.test",
        getToken: () => "token_123",
      })

      await client.GET("/events")

      expect(fetchMock).toHaveBeenCalledTimes(1)
      const request = fetchMock.mock.calls[0]?.[0] as Request
      expect(request.url).toBe("http://api.test/events")
      expect(request.headers.get("Authorization")).toBe("Bearer token_123")
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it("omits the authorization header when no token is available", async () => {
    const fetchMock = vi.fn((_input: RequestInfo | URL, _init?: RequestInit) =>
      Promise.resolve(jsonResponse({ data: { ok: true }, error: null }))
    )
    vi.stubGlobal("fetch", fetchMock)

    try {
      const client = createApiClient({
        baseUrl: "http://api.test",
        getToken: () => null,
      })

      await client.GET("/events")

      const request = fetchMock.mock.calls[0]?.[0] as Request
      expect(request.headers.has("Authorization")).toBe(false)
    } finally {
      vi.unstubAllGlobals()
    }
  })
})
