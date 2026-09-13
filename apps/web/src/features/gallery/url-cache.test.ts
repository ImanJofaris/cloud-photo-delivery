import { describe, expect, it, vi } from "vitest"

import { createSignedUrlCache, URL_EXPIRY_MARGIN_MS } from "./url-cache"

function signed(url: string, expiresIn = 300) {
  return Promise.resolve({ url, expiresIn })
}

describe("signed URL cache", () => {
  it("serves a cached url without refetching", async () => {
    const fetcher = vi.fn((photoId: string, variant: string) =>
      signed(`https://r2.test/${photoId}-${variant}`)
    )
    const cache = createSignedUrlCache(fetcher)

    expect(await cache.fetch("p1", "large")).toBe("https://r2.test/p1-large")
    expect(await cache.fetch("p1", "large")).toBe("https://r2.test/p1-large")
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(cache.size()).toBe(1)
  })

  it("refetches once the cached url enters the expiry margin", async () => {
    let now = 0
    const fetcher = vi.fn(() =>
      signed(`https://r2.test/v${fetcher.mock.calls.length}`)
    )
    const cache = createSignedUrlCache(fetcher, () => now)

    await cache.fetch("p1", "large")

    now = 300_000 - URL_EXPIRY_MARGIN_MS - 1
    expect(await cache.fetch("p1", "large")).toBe("https://r2.test/v1")
    expect(fetcher).toHaveBeenCalledTimes(1)

    now = 300_000 - URL_EXPIRY_MARGIN_MS
    expect(await cache.fetch("p1", "large")).toBe("https://r2.test/v2")
    expect(fetcher).toHaveBeenCalledTimes(2)
  })

  it("dedupes concurrent fetches for the same photo and variant", async () => {
    let resolveFetch: (value: { url: string; expiresIn: number }) => void =
      () => {}
    const fetcher = vi.fn(
      () =>
        new Promise<{ url: string; expiresIn: number }>((resolve) => {
          resolveFetch = resolve
        })
    )
    const cache = createSignedUrlCache(fetcher)

    const first = cache.fetch("p1", "thumbnail")
    const second = cache.fetch("p1", "thumbnail")
    resolveFetch({ url: "https://r2.test/thumb.jpg", expiresIn: 300 })

    expect(await Promise.all([first, second])).toEqual([
      "https://r2.test/thumb.jpg",
      "https://r2.test/thumb.jpg",
    ])
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it("keeps urls in memory only", async () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem")
    const cache = createSignedUrlCache(
      vi.fn(() => signed("https://r2.test/a.jpg"))
    )

    await cache.fetch("p1", "thumbnail")

    expect(setItem).not.toHaveBeenCalled()
    const fresh = createSignedUrlCache(
      vi.fn(() => signed("https://r2.test/b.jpg"))
    )
    expect(await fresh.fetch("p1", "thumbnail")).toBe("https://r2.test/b.jpg")
    setItem.mockRestore()
  })
})
