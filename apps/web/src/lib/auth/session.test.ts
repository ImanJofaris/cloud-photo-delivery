import { beforeEach, describe, expect, it, vi } from "vitest"

import {
  getAccessToken,
  getSnapshot,
  refreshSession,
  resetSessionForTests,
  setSession,
  clearSession,
} from "./session"

function jsonResponse(body: unknown, status = 200) {
  return Response.json(body, { status })
}

describe("session", () => {
  beforeEach(() => {
    resetSessionForTests()
    vi.unstubAllGlobals()
  })

  it("single-flights concurrent refreshes", async () => {
    let release: () => void = () => {}
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const fetchMock = vi.fn(async () => {
      await gate
      return jsonResponse({
        data: {
          accessToken: "access_1",
          user: { id: "u1", email: "a@b.com" },
        },
      })
    })
    vi.stubGlobal("fetch", fetchMock)

    const first = refreshSession()
    const second = refreshSession()
    release()

    await expect(Promise.all([first, second])).resolves.toEqual([true, true])
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(getSnapshot().status).toBe("authenticated")
    expect(getAccessToken()).toBe("access_1")
  })

  it("clears the session when refresh fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse({ data: null, error: {} }, 401))
    )

    await expect(refreshSession()).resolves.toBe(false)
    expect(getSnapshot().status).toBe("anonymous")
    expect(getAccessToken()).toBeNull()
  })

  it("sets and clears an in-memory session", () => {
    setSession({
      accessToken: "access_2",
      user: { id: "u2", email: "c@d.com" },
    })
    expect(getAccessToken()).toBe("access_2")
    expect(getSnapshot().user?.email).toBe("c@d.com")

    clearSession()
    expect(getSnapshot().status).toBe("anonymous")
    expect(getAccessToken()).toBeNull()
  })
})
