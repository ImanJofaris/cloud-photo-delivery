const STORAGE_PREFIX = "cpd.gallery.unlock."
const EXPIRY_MARGIN_MS = 5_000

type StoredUnlock = {
  token: string
  expiresAt: number
}

const memory = new Map<string, StoredUnlock>()
const listeners = new Set<() => void>()

function emit() {
  for (const listener of listeners) listener()
}

export function subscribeUnlock(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

function storageKey(slug: string) {
  return `${STORAGE_PREFIX}${slug}`
}

function sessionStorageOrNull(): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}

function isStoredUnlock(value: unknown): value is StoredUnlock {
  if (!value || typeof value !== "object") return false
  const entry = value as Partial<StoredUnlock>
  return typeof entry.token === "string" && typeof entry.expiresAt === "number"
}

function isFresh(entry: StoredUnlock, now: number) {
  return entry.expiresAt - EXPIRY_MARGIN_MS > now
}

export function storeUnlockToken(
  slug: string,
  token: string,
  expiresInSeconds: number
) {
  const entry: StoredUnlock = {
    token,
    expiresAt: Date.now() + expiresInSeconds * 1000,
  }
  memory.set(slug, entry)
  try {
    sessionStorageOrNull()?.setItem(storageKey(slug), JSON.stringify(entry))
  } catch {
    // Private mode or quota: the in-memory mirror still unlocks this tab.
  }
  emit()
}

export function readUnlockToken(slug: string): string | null {
  const now = Date.now()
  const cached = memory.get(slug)
  if (cached) {
    if (isFresh(cached, now)) return cached.token
    memory.delete(slug)
  }

  const storage = sessionStorageOrNull()
  if (!storage) return null
  try {
    const raw = storage.getItem(storageKey(slug))
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (!isStoredUnlock(parsed) || !isFresh(parsed, now)) {
      storage.removeItem(storageKey(slug))
      return null
    }
    memory.set(slug, parsed)
    return parsed.token
  } catch {
    return null
  }
}

export function clearUnlockToken(slug: string) {
  memory.delete(slug)
  try {
    sessionStorageOrNull()?.removeItem(storageKey(slug))
  } catch {
    // Nothing to clear.
  }
  emit()
}

export function unlockHeader(slug: string): Record<string, string> {
  const token = readUnlockToken(slug)
  return token ? { "X-Gallery-Unlock": token } : {}
}
