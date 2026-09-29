import { ApiError, type components } from "@workspace/api-client"

import type { SignedUrlFetcher, UrlVariant } from "./url-cache"

type SignedURL = components["schemas"]["SignedURL"]

export type BatchUrlFetcher = (
  variant: UrlVariant,
  photoIds: string[]
) => Promise<Record<string, SignedURL>>

export const URL_BATCH_DELAY_MS = 50
export const URL_BATCH_MAX_SIZE = 100

type UrlWaiter = {
  resolve: (value: SignedURL) => void
  reject: (error: unknown) => void
}

type PendingEntry = {
  photoId: string
  variant: UrlVariant
  waiters: UrlWaiter[]
}

function keyOf(photoId: string, variant: UrlVariant) {
  return `${photoId}:${variant}`
}

/**
 * Groups per-photo URL lookups into batch requests so a gallery page signs
 * many photos per HTTP request. The returned fetcher keeps the single-photo
 * signature of `SignedUrlFetcher`, so it drops into `createSignedUrlCache`
 * without changing callers.
 */
export function createBatchedUrlFetcher(
  fetchBatch: BatchUrlFetcher,
  options: { delayMs?: number; maxBatchSize?: number } = {}
): SignedUrlFetcher {
  const delayMs = options.delayMs ?? URL_BATCH_DELAY_MS
  const maxBatchSize = options.maxBatchSize ?? URL_BATCH_MAX_SIZE

  const pending = new Map<string, PendingEntry>()
  let timer: ReturnType<typeof setTimeout> | null = null

  async function dispatch(variant: UrlVariant, entries: PendingEntry[]) {
    try {
      const urls = await fetchBatch(
        variant,
        entries.map((entry) => entry.photoId)
      )
      for (const entry of entries) {
        const result = urls[entry.photoId]
        if (result) {
          for (const waiter of entry.waiters) waiter.resolve(result)
        } else {
          const error = new ApiError(
            "VARIANT_UNAVAILABLE",
            "Variant unavailable",
            404
          )
          for (const waiter of entry.waiters) waiter.reject(error)
        }
      }
    } catch (error) {
      for (const entry of entries) {
        for (const waiter of entry.waiters) waiter.reject(error)
      }
    }
  }

  function flush() {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
    const groups = new Map<UrlVariant, PendingEntry[]>()
    for (const entry of pending.values()) {
      const group = groups.get(entry.variant)
      if (group) group.push(entry)
      else groups.set(entry.variant, [entry])
    }
    pending.clear()

    for (const [variant, entries] of groups) {
      for (let i = 0; i < entries.length; i += maxBatchSize) {
        void dispatch(variant, entries.slice(i, i + maxBatchSize))
      }
    }
  }

  function schedule() {
    if (pending.size >= maxBatchSize) {
      flush()
      return
    }
    if (timer !== null) return
    timer = setTimeout(flush, delayMs)
  }

  return function fetch(photoId: string, variant: UrlVariant) {
    return new Promise<SignedURL>((resolve, reject) => {
      const key = keyOf(photoId, variant)
      const waiter = { resolve, reject }
      const existing = pending.get(key)
      if (existing) {
        // Keep every caller waiting on the same key; settling a new promise
        // with a map overwrite would strand the earlier one forever.
        existing.waiters.push(waiter)
      } else {
        pending.set(key, { photoId, variant, waiters: [waiter] })
      }
      schedule()
    })
  }
}
