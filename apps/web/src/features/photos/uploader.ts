import { ApiError, type components } from "@workspace/api-client"

import { isNotFound, photoErrorMessage } from "./errors"
import { uploadKindFor, validateFile } from "./schema"
import type { UploadRegistry } from "./storage"
import {
  UploadTransportError,
  type PutTransport,
} from "./transport"

type InitializeResult = components["schemas"]["InitializeUpload"]
type UploadStatusDto = components["schemas"]["UploadStatus"]
type PresignedUploadDto = components["schemas"]["PresignedUpload"]
type PartURL = components["schemas"]["PartURL"]

export type UploadKind = components["schemas"]["UploadKind"]

export type UploadItemStatus =
  | "queued"
  | "uploading"
  | "processing"
  | "ready"
  | "failed"
  | "interrupted"

export type UploadItem = {
  key: string
  filename: string
  size: number
  mimeType: string
  status: UploadItemStatus
  progress: number
  attempts: number
  error: string | null
  photoId: string | null
  uploadKind: UploadKind | null
  fromRegistry: boolean
  file: File | null
  partSize: number | null
  uploadedParts: Set<number>
  partETags: Map<number, string>
}

export type UploadDeps = {
  initialize: (file: File, idempotencyKey: string) => Promise<InitializeResult>
  rePresign: (photoId: string) => Promise<PresignedUploadDto>
  uploadStatus: (photoId: string) => Promise<UploadStatusDto>
  partUrls: (
    photoId: string,
    partNumbers: number[]
  ) => Promise<{ partSize: number; parts: PartURL[] }>
  completeMultipart: (
    photoId: string,
    parts: { partNumber: number; etag: string }[]
  ) => Promise<void>
  abortMultipart: (photoId: string) => Promise<void>
  complete: (photoId: string, idempotencyKey: string) => Promise<unknown>
  deletePhoto: (photoId: string) => Promise<void>
  put: PutTransport
  registry?: UploadRegistry
  concurrency?: number
  maxAttempts?: number
  sleep?: (ms: number) => Promise<void>
  randomUUID?: () => string
  onChanged?: (item: UploadItem) => void
}

const DEFAULT_CONCURRENCY = 3
const DEFAULT_MAX_ATTEMPTS = 3
const DEFAULT_PART_SIZE = 10 * 1024 * 1024
const PART_CONCURRENCY = 3
const PART_URL_BATCH = 100

export function isAbortError(error: unknown): boolean {
  return (
    error instanceof DOMException && error.name === "AbortError"
  )
}

export function isRetryableError(error: unknown): boolean {
  if (isAbortError(error)) return false
  if (error instanceof UploadTransportError) {
    return error.status === 0 || error.status === 429 || error.status >= 500
  }
  if (error instanceof ApiError) {
    return error.status === 429 || error.status >= 500
  }
  return true
}

function backoffMs(attempt: number): number {
  return Math.min(1000 * 2 ** (attempt - 1), 8000)
}

function chunk<T>(items: T[], size: number): T[][] {
  const out: T[][] = []
  for (let i = 0; i < items.length; i += size) {
    out.push(items.slice(i, i + size))
  }
  return out
}

async function runPool<T>(
  items: T[],
  limit: number,
  worker: (item: T) => Promise<void>
): Promise<void> {
  let next = 0
  const workers = Array.from(
    { length: Math.min(limit, items.length) },
    async () => {
      while (true) {
        const index = next
        next += 1
        if (index >= items.length) return
        await worker(items[index])
      }
    }
  )
  const results = await Promise.allSettled(workers)
  const failed = results.find((result) => result.status === "rejected")
  if (failed?.status === "rejected") {
    throw failed.reason
  }
}

export class UploadQueue {
  private items = new Map<string, UploadItem>()
  private listeners = new Set<() => void>()
  private controllers = new Map<string, AbortController>()
  private active = 0
  private cached: UploadItem[] = []

  constructor(
    private readonly eventId: string,
    private readonly deps: UploadDeps
  ) {}

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  snapshot = (): UploadItem[] => this.cached

  getItems(): UploadItem[] {
    return this.cached
  }

  addFiles(files: File[]): { filename: string; reason: string }[] {
    const rejected: { filename: string; reason: string }[] = []
    for (const file of files) {
      const reason = validateFile(file)
      if (reason) {
        rejected.push({ filename: file.name, reason })
        continue
      }
      const key = this.newKey()
      this.items.set(key, {
        key,
        filename: file.name,
        size: file.size,
        mimeType: file.type,
        status: "queued",
        progress: 0,
        attempts: 0,
        error: null,
        photoId: null,
        uploadKind: uploadKindFor(file.size),
        fromRegistry: false,
        file,
        partSize: null,
        uploadedParts: new Set(),
        partETags: new Map(),
      })
    }
    this.notify()
    this.pump()
    return rejected
  }

  retry(key: string) {
    const item = this.items.get(key)
    if (!item || !item.file) return
    item.status = "queued"
    item.error = null
    item.attempts = 0
    this.notify()
    this.pump()
  }

  async cancel(key: string): Promise<void> {
    this.controllers.get(key)?.abort()
    const item = this.items.get(key)
    if (!item) return
    if (item.photoId && item.uploadKind === "multipart") {
      try {
        await this.deps.abortMultipart(item.photoId)
      } catch {
        // The multipart upload may already have completed; deletion below
        // still removes the row and any stored objects.
      }
    }
    if (item.photoId) {
      try {
        await this.deps.deletePhoto(item.photoId)
      } catch (error) {
        if (!isNotFound(error)) throw error
      }
    }
    this.deps.registry?.remove(key)
    this.items.delete(key)
    this.notify()
  }

  async discard(key: string): Promise<void> {
    const item = this.items.get(key)
    if (!item) return
    if (item.photoId) {
      try {
        await this.deps.deletePhoto(item.photoId)
      } catch (error) {
        if (!isNotFound(error)) throw error
      }
    }
    this.deps.registry?.remove(key)
    this.items.delete(key)
    this.notify()
  }

  abortAll() {
    for (const controller of this.controllers.values()) {
      controller.abort()
    }
    for (const item of this.items.values()) {
      if (item.status === "uploading") {
        item.status = "queued"
      }
    }
    this.notify()
  }

  // applyPhotoStatuses resolves queue rows from the authoritative list once
  // the processing worker reaches a terminal state.
  applyPhotoStatuses(
    photos: { id: string; status: string; errorMessage: string | null }[]
  ) {
    let changed = false
    for (const item of this.items.values()) {
      if (!item.photoId || item.status !== "processing") continue
      const photo = photos.find((candidate) => candidate.id === item.photoId)
      if (!photo) continue
      if (photo.status === "READY") {
        item.status = "ready"
        item.error = null
        changed = true
      } else if (photo.status === "FAILED") {
        item.status = "failed"
        item.error = photo.errorMessage ?? "Processing failed."
        changed = true
      }
    }
    if (changed) this.notify()
  }

  // reconcile restores upload state that survived a reload. Bytes cannot be
  // re-read after a reload, so UPLOADING rows become interrupted items the
  // user can discard; PROCESSING/READY rows simply leave the queue.
  async reconcile(): Promise<void> {
    const registry = this.deps.registry
    if (!registry) return
    for (const entry of registry.list(this.eventId)) {
      if (this.items.has(entry.key)) continue
      if (!entry.photoId) {
        registry.remove(entry.key)
        continue
      }
      try {
        const status = await this.deps.uploadStatus(entry.photoId)
        if (status.status === "READY" || status.status === "PROCESSING") {
          registry.remove(entry.key)
          continue
        }
        const item: UploadItem = {
          key: entry.key,
          filename: status.filename || entry.filename,
          size: Number(status.fileSize ?? 0),
          mimeType: status.mimeType || "image/jpeg",
          status: "interrupted",
          progress: 0,
          attempts: 0,
          error: "Upload was interrupted. Select the file again to retry.",
          photoId: entry.photoId,
          uploadKind: status.uploadKind ?? entry.uploadKind,
          fromRegistry: true,
          file: null,
          partSize: null,
          uploadedParts: new Set(),
          partETags: new Map(),
        }
        if (status.status === "FAILED") {
          item.status = "failed"
          item.error = status.errorMessage || "Processing failed."
        }
        this.items.set(entry.key, item)
      } catch (error) {
        if (isNotFound(error)) {
          registry.remove(entry.key)
        }
      }
    }
    this.notify()
  }

  private newKey(): string {
    return this.deps.randomUUID?.() ?? crypto.randomUUID()
  }

  private sleep(ms: number): Promise<void> {
    return this.deps.sleep?.(ms) ?? new Promise((resolve) => setTimeout(resolve, ms))
  }

  private pump() {
    const limit = this.deps.concurrency ?? DEFAULT_CONCURRENCY
    for (const item of this.items.values()) {
      if (this.active >= limit) break
      if (item.status !== "queued" || !item.file) continue
      this.active += 1
      item.status = "uploading"
      void this.process(item)
        .catch(() => {
          // process handles its own failures
        })
        .finally(() => {
          this.active -= 1
          this.notify()
          this.pump()
        })
    }
    this.notify()
  }

  private async process(item: UploadItem): Promise<void> {
    const file = item.file
    if (!file) return
    const maxAttempts = this.deps.maxAttempts ?? DEFAULT_MAX_ATTEMPTS
    item.error = null
    item.progress = 0

    while (true) {
      const controller = new AbortController()
      this.controllers.set(item.key, controller)
      try {
        await this.attempt(item, file, controller.signal)
        this.controllers.delete(item.key)
        item.status = "processing"
        item.progress = 100
        item.error = null
        this.persist(item)
        this.notify()
        this.deps.onChanged?.(item)
        return
      } catch (error) {
        this.controllers.delete(item.key)
        if (isAbortError(error)) {
          item.status = "queued"
          this.notify()
          return
        }
        item.attempts += 1
        if (isRetryableError(error) && item.attempts < maxAttempts) {
          item.status = "queued"
          item.error = "Connection problem. Retrying..."
          this.notify()
          await this.sleep(backoffMs(item.attempts))
          item.status = "uploading"
          this.notify()
          continue
        }
        item.status = "failed"
        item.error = photoErrorMessage(error)
        this.persist(item)
        this.notify()
        this.deps.onChanged?.(item)
        return
      }
    }
  }

  private async attempt(
    item: UploadItem,
    file: File,
    signal: AbortSignal
  ): Promise<void> {
    if (!item.photoId) {
      const init = await this.deps.initialize(file, this.newKey())
      item.photoId = init.photoId
      item.uploadKind = init.uploadKind
      if (init.partSize) item.partSize = Number(init.partSize)
      this.persist(item)
      if (init.uploadKind === "multipart") {
        await this.putMultipart(item, file, signal)
      } else {
        await this.putSimple(item, file, init.uploadUrl, signal)
      }
    } else if (item.uploadKind === "multipart") {
      await this.putMultipart(item, file, signal)
    } else {
      const fresh = await this.deps.rePresign(item.photoId)
      await this.putSimple(item, file, fresh.uploadUrl, signal)
    }
    await this.deps.complete(item.photoId, this.newKey())
  }

  private async putSimple(
    item: UploadItem,
    file: File,
    url: string,
    signal: AbortSignal
  ): Promise<void> {
    await this.deps.put({
      url,
      body: file,
      signal,
      onProgress: (loaded, total) => {
        item.progress =
          total > 0 ? Math.min(100, Math.round((loaded / total) * 100)) : 0
        this.notify()
      },
    })
  }

  private async putMultipart(
    item: UploadItem,
    file: File,
    signal: AbortSignal
  ): Promise<void> {
    const photoId = item.photoId
    if (!photoId) return
    const partSize = item.partSize ?? DEFAULT_PART_SIZE
    const totalParts = Math.max(1, Math.ceil(file.size / partSize))
    const missing: number[] = []
    for (let n = 1; n <= totalParts; n += 1) {
      if (!item.uploadedParts.has(n)) missing.push(n)
    }

    const urls = new Map<number, string>()
    for (const batch of chunk(missing, PART_URL_BATCH)) {
      const result = await this.deps.partUrls(photoId, batch)
      if (result.partSize) item.partSize = Number(result.partSize)
      for (const part of result.parts) {
        urls.set(part.partNumber, part.url)
      }
    }

    const completedBytes = () => {
      let bytes = 0
      for (const n of item.uploadedParts) {
        bytes += Math.min(partSize, file.size - (n - 1) * partSize)
      }
      return bytes
    }

    await runPool(missing, PART_CONCURRENCY, async (n) => {
      const url = urls.get(n)
      if (!url) throw new Error(`Missing upload URL for part ${n}.`)
      const slice = file.slice(
        (n - 1) * partSize,
        Math.min(n * partSize, file.size)
      )
      const base = completedBytes()
      const { etag } = await this.deps.put({
        url,
        body: slice,
        signal,
        onProgress: (loaded) => {
          item.progress = Math.min(
            99,
            Math.round(((base + loaded) / file.size) * 100)
          )
          this.notify()
        },
      })
      if (!etag) {
        throw new Error(
          "The storage provider did not return an upload signature (ETag)."
        )
      }
      item.partETags.set(n, etag)
      item.uploadedParts.add(n)
      this.notify()
    })

    const parts = [...item.uploadedParts]
      .sort((a, b) => a - b)
      .map((partNumber) => ({
        partNumber,
        etag: item.partETags.get(partNumber) ?? "",
      }))
    await this.deps.completeMultipart(photoId, parts)
  }

  private persist(item: UploadItem) {
    this.deps.registry?.upsert({
      key: item.key,
      eventId: this.eventId,
      photoId: item.photoId,
      uploadKind: item.uploadKind,
      filename: item.filename,
      updatedAt: Date.now(),
    })
  }

  private notify() {
    this.cached = [...this.items.values()]
    for (const listener of this.listeners) {
      listener()
    }
  }
}
