export type UploadRegistryEntry = {
  key: string
  eventId: string
  photoId: string | null
  uploadKind: "simple" | "multipart" | null
  filename: string
  updatedAt: number
}

export interface UploadRegistry {
  list(eventId: string): UploadRegistryEntry[]
  upsert(entry: UploadRegistryEntry): void
  remove(key: string): void
}

const STORAGE_KEY = "cpd.uploads.v1"
const MAX_AGE_MS = 24 * 60 * 60 * 1000

function isEntry(value: unknown): value is UploadRegistryEntry {
  if (!value || typeof value !== "object") return false
  const entry = value as Partial<UploadRegistryEntry>
  return (
    typeof entry.key === "string" &&
    typeof entry.eventId === "string" &&
    typeof entry.filename === "string" &&
    typeof entry.updatedAt === "number"
  )
}

function readAll(storage: Storage | null): UploadRegistryEntry[] {
  if (!storage) return []
  try {
    const raw = storage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(isEntry)
  } catch {
    return []
  }
}

function writeAll(storage: Storage | null, entries: UploadRegistryEntry[]) {
  if (!storage) return
  try {
    storage.setItem(STORAGE_KEY, JSON.stringify(entries))
  } catch {
    // Storage can be unavailable (private mode, quota); uploads still work.
  }
}

export function createLocalStorageRegistry(storage?: Storage): UploadRegistry {
  const resolve = (): Storage | null => {
    if (storage) return storage
    return typeof window === "undefined" ? null : window.localStorage
  }
  return {
    list(eventId) {
      const cutoff = Date.now() - MAX_AGE_MS
      const entries = readAll(resolve())
      const fresh = entries.filter((entry) => entry.updatedAt >= cutoff)
      if (fresh.length !== entries.length) writeAll(resolve(), fresh)
      return fresh.filter((entry) => entry.eventId === eventId)
    },
    upsert(entry) {
      const entries = readAll(resolve()).filter((e) => e.key !== entry.key)
      entries.push(entry)
      writeAll(resolve(), entries)
    },
    remove(key) {
      writeAll(
        resolve(),
        readAll(resolve()).filter((e) => e.key !== key)
      )
    },
  }
}
