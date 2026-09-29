import { describe, expect, it, vi } from "vitest"

import {
  createBatchedUrlFetcher,
  URL_BATCH_DELAY_MS,
  URL_BATCH_MAX_SIZE,
} from "./batch-url-fetcher"

function signed(url: string, expiresIn = 300) {
  return { url, expiresIn }
}

describe("batched URL fetcher", () => {
  it("coalesces concurrent lookups into one batch request", async () => {
    vi.useFakeTimers()
    try {
      const fetchBatch = vi.fn(async (variant: string, photoIds: string[]) =>
        Object.fromEntries(
          photoIds.map((id) => [id, signed(`https://r2.test/${id}-${variant}`)])
        )
      )
      const fetcher = createBatchedUrlFetcher(fetchBatch)

      const first = fetcher("p1", "thumbnail")
      const second = fetcher("p2", "thumbnail")
      vi.advanceTimersByTime(URL_BATCH_DELAY_MS)

      await expect(Promise.all([first, second])).resolves.toEqual([
        signed("https://r2.test/p1-thumbnail"),
        signed("https://r2.test/p2-thumbnail"),
      ])
      expect(fetchBatch).toHaveBeenCalledTimes(1)
      expect(fetchBatch).toHaveBeenCalledWith("thumbnail", ["p1", "p2"])
    } finally {
      vi.useRealTimers()
    }
  })

  it("settles every caller when the same key is requested twice", async () => {
    vi.useFakeTimers()
    try {
      const fetchBatch = vi.fn(async (variant: string, photoIds: string[]) =>
        Object.fromEntries(
          photoIds.map((id) => [id, signed(`https://r2.test/${id}-${variant}`)])
        )
      )
      const fetcher = createBatchedUrlFetcher(fetchBatch)

      const first = fetcher("p1", "thumbnail")
      const second = fetcher("p1", "thumbnail")
      vi.advanceTimersByTime(URL_BATCH_DELAY_MS)

      await expect(Promise.all([first, second])).resolves.toEqual([
        signed("https://r2.test/p1-thumbnail"),
        signed("https://r2.test/p1-thumbnail"),
      ])
      expect(fetchBatch).toHaveBeenCalledTimes(1)
      expect(fetchBatch).toHaveBeenCalledWith("thumbnail", ["p1"])
    } finally {
      vi.useRealTimers()
    }
  })

  it("keeps variants in separate batch requests", async () => {
    vi.useFakeTimers()
    try {
      const fetchBatch = vi.fn(async (variant: string, photoIds: string[]) =>
        Object.fromEntries(
          photoIds.map((id) => [id, signed(`https://r2.test/${id}-${variant}`)])
        )
      )
      const fetcher = createBatchedUrlFetcher(fetchBatch)

      const tile = fetcher("p1", "thumbnail")
      const viewer = fetcher("p1", "large")
      vi.advanceTimersByTime(URL_BATCH_DELAY_MS)

      await Promise.all([tile, viewer])
      expect(fetchBatch).toHaveBeenCalledTimes(2)
      expect(fetchBatch).toHaveBeenCalledWith("thumbnail", ["p1"])
      expect(fetchBatch).toHaveBeenCalledWith("large", ["p1"])
    } finally {
      vi.useRealTimers()
    }
  })

  it("rejects photos the response omits with VARIANT_UNAVAILABLE", async () => {
    vi.useFakeTimers()
    try {
      const fetcher = createBatchedUrlFetcher(async () => ({
        p1: signed("https://r2.test/p1.jpg"),
      }))

      const ok = fetcher("p1", "thumbnail")
      const missing = fetcher("p2", "thumbnail")
      const missingAssertion = expect(missing).rejects.toMatchObject({
        code: "VARIANT_UNAVAILABLE",
        status: 404,
      })
      vi.advanceTimersByTime(URL_BATCH_DELAY_MS)

      await expect(ok).resolves.toEqual(signed("https://r2.test/p1.jpg"))
      await missingAssertion
    } finally {
      vi.useRealTimers()
    }
  })

  it("rejects the whole batch when the request fails", async () => {
    vi.useFakeTimers()
    try {
      const failure = new Error("network down")
      const fetcher = createBatchedUrlFetcher(async () => {
        throw failure
      })

      const assertion = expect(fetcher("p1", "thumbnail")).rejects.toBe(failure)
      vi.advanceTimersByTime(URL_BATCH_DELAY_MS)
      await assertion
    } finally {
      vi.useRealTimers()
    }
  })

  it("flushes immediately once a full batch is queued", async () => {
    const fetchBatch = vi.fn(async (variant: string, photoIds: string[]) =>
      Object.fromEntries(
        photoIds.map((id) => [id, signed(`https://r2.test/${id}-${variant}`)])
      )
    )
    const fetcher = createBatchedUrlFetcher(fetchBatch)

    const calls = Array.from({ length: URL_BATCH_MAX_SIZE }, (_, index) =>
      fetcher(`p${index}`, "thumbnail")
    )
    await Promise.all(calls)

    expect(fetchBatch).toHaveBeenCalledTimes(1)
    expect(fetchBatch.mock.calls[0]?.[1]).toHaveLength(URL_BATCH_MAX_SIZE)
  })
})
