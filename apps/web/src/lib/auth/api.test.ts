import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError } from "@workspace/api-client"

import { apiBlobCall, resetApiClientForTests } from "./api"
import { getAccessToken, resetSessionForTests, setSession } from "./session"

function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === "string") return input
  if (input instanceof URL) return input.toString()
  return input.url
}

function blobResponse(body: string, status = 200): Response {
  return new Response(body, {
    status,
    headers: { "content-type": "image/png" },
  })
}

function errorResponse(code: string, status: number): Response {
  return new Response(
    JSON.stringify({ data: null, error: { code, message: code } }),
    { status, headers: { "content-type": "application/json" } }
  )
}

type BlobClient = Parameters<typeof apiBlobCall>[0] extends (
  client: infer C
) => unknown
  ? C
  : never

function qrCall(client: BlobClient) {
  return client.GET("/events/{eventID}/qr.png", {
    params: { path: { eventID: "event-1" } },
    parseAs: "blob",
  })
}

describe("apiBlobCall", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080"
    resetSessionForTests()
    vi.unstubAllGlobals()
    resetApiClientForTests()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("returns the response blob on success", async () => {
    setSession({ accessToken: "access_1" })
    vi.stubGlobal("fetch", vi.fn(async () => blobResponse("png-bytes")))

    const blob = await apiBlobCall((client) => qrCall(client))

    expect(blob).toBeInstanceOf(Blob)
    await expect(blob.text()).resolves.toBe("png-bytes")
  })

  it("refreshes once on 401 and retries with the new token", async () => {
    setSession({ accessToken: "expired" })
    const authorizations: (string | null)[] = []
    let qrCalls = 0

    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = requestUrl(input)
        if (url.includes("/api/auth/refresh")) {
          return Response.json({
            data: {
              accessToken: "fresh",
              user: { id: "u1", email: "a@b.com" },
            },
          })
        }
        const request =
          input instanceof Request ? input : new Request(url, init)
        authorizations.push(request.headers.get("Authorization"))
        qrCalls += 1
        if (qrCalls === 1) return errorResponse("UNAUTHENTICATED", 401)
        return blobResponse("retried")
      })
    )

    const blob = await apiBlobCall((client) => qrCall(client))

    await expect(blob.text()).resolves.toBe("retried")
    expect(qrCalls).toBe(2)
    expect(authorizations).toEqual(["Bearer expired", "Bearer fresh"])
    expect(getAccessToken()).toBe("fresh")
  })

  it("maps an error envelope to ApiError without refreshing twice", async () => {
    setSession({ accessToken: "access_1" })
    const fetchMock = vi.fn(async () => errorResponse("EVENT_NOT_FOUND", 404))
    vi.stubGlobal("fetch", fetchMock)

    await expect(apiBlobCall((client) => qrCall(client))).rejects.toMatchObject({
      code: "EVENT_NOT_FOUND",
      status: 404,
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it("throws an ApiError for non-envelope failures", async () => {
    setSession({ accessToken: "access_1" })
    vi.stubGlobal("fetch", vi.fn(async () => new Response("nope", { status: 500 })))

    await expect(apiBlobCall((client) => qrCall(client))).rejects.toBeInstanceOf(
      ApiError
    )
  })
})
