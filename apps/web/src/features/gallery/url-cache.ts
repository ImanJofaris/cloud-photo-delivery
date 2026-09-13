import type { components } from "@workspace/api-client"

type SignedURL = components["schemas"]["SignedURL"]

export type UrlVariant = "thumbnail" | "medium" | "large" | "original"

export type SignedUrlFetcher = (
  photoId: string,
  variant: UrlVariant
) => Promise<SignedURL>

export const URL_EXPIRY_MARGIN_MS = 30_000

type CacheEntry = {
  url: string
  expiresAt: number
}

export function createSignedUrlCache(
  fetcher: SignedUrlFetcher,
  now: () => number = Date.now
) {
  const entries = new Map<string, CacheEntry>()
  const inflight = new Map<string, Promise<SignedURL>>()

  function keyOf(photoId: string, variant: UrlVariant) {
    return `${photoId}:${variant}`
  }

  async function fetch(
    photoId: string,
    variant: UrlVariant,
    options?: { force?: boolean }
  ): Promise<string> {
    const key = keyOf(photoId, variant)

    if (options?.force) {
      entries.delete(key)
      inflight.delete(key)
    } else {
      const cached = entries.get(key)
      if (cached && cached.expiresAt > now()) return cached.url
    }

    let pending = inflight.get(key)
    if (!pending) {
      pending = fetcher(photoId, variant)
      inflight.set(key, pending)
    }

    try {
      const result = await pending
      entries.set(key, {
        url: result.url,
        expiresAt: now() + result.expiresIn * 1000 - URL_EXPIRY_MARGIN_MS,
      })
      return result.url
    } finally {
      if (inflight.get(key) === pending) inflight.delete(key)
    }
  }

  function clear() {
    entries.clear()
    inflight.clear()
  }

  function size() {
    return entries.size
  }

  return { fetch, clear, size }
}

export type SignedUrlCache = ReturnType<typeof createSignedUrlCache>
