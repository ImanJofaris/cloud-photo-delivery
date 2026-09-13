import { renderHook } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { ApiError } from "@workspace/api-client"

import { useUnlockGallery } from "./api"
import { createQueryWrapper, jsonResponse } from "./test-utils"
import {
  clearUnlockToken,
  readUnlockToken,
  storeUnlockToken,
  subscribeUnlock,
  unlockHeader,
} from "./unlock"

const SLUG = "wedding"
const STORAGE_KEY = `cpd.gallery.unlock.${SLUG}`

describe("gallery unlock tokens", () => {
  beforeEach(() => {
    sessionStorage.clear()
    clearUnlockToken(SLUG)
    clearUnlockToken("other")
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it("stores a token per slug", () => {
    storeUnlockToken(SLUG, "token-a", 1800)
    storeUnlockToken("other", "token-b", 1800)

    expect(readUnlockToken(SLUG)).toBe("token-a")
    expect(readUnlockToken("other")).toBe("token-b")
    expect(readUnlockToken("missing")).toBeNull()
  })

  it("expires tokens after their ttl", () => {
    vi.useFakeTimers()
    storeUnlockToken(SLUG, "token", 30)
    expect(readUnlockToken(SLUG)).toBe("token")

    vi.advanceTimersByTime(30_000)
    expect(readUnlockToken(SLUG)).toBeNull()
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it("clears tokens on demand", () => {
    storeUnlockToken(SLUG, "token", 1800)
    clearUnlockToken(SLUG)
    expect(readUnlockToken(SLUG)).toBeNull()
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it("notifies subscribers when tokens change", () => {
    const listener = vi.fn()
    const unsubscribe = subscribeUnlock(listener)

    storeUnlockToken(SLUG, "token", 1800)
    clearUnlockToken(SLUG)
    unsubscribe()
    storeUnlockToken(SLUG, "token", 1800)

    expect(listener).toHaveBeenCalledTimes(2)
  })

  it("returns an unlock header only when a token exists", () => {
    expect(unlockHeader(SLUG)).toEqual({})

    storeUnlockToken(SLUG, "token", 1800)
    expect(unlockHeader(SLUG)).toEqual({ "X-Gallery-Unlock": "token" })
  })

  it("clears the token when a request is rejected with 401", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080"
    storeUnlockToken(SLUG, "expired-token", 1800)

    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(
          {
            data: null,
            error: { code: "UNAUTHORIZED", message: "Invalid token" },
          },
          401
        )
      )
    )

    const { result } = renderHook(() => useUnlockGallery(SLUG), {
      wrapper: createQueryWrapper(),
    })

    await expect(result.current.mutateAsync("wrong")).rejects.toBeInstanceOf(
      ApiError
    )
    expect(readUnlockToken(SLUG)).toBeNull()
  })
})
