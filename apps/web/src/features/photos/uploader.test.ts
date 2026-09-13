import { describe, expect, it, vi } from "vitest"

import type { components } from "@workspace/api-client"

import type { UploadRegistry, UploadRegistryEntry } from "./storage"
import type { PutTransport } from "./transport"
import { UploadTransportError } from "./transport"
import {
  UploadQueue,
  type UploadDeps,
  type UploadItem,
} from "./uploader"

type InitializeUpload = components["schemas"]["InitializeUpload"]
type UploadStatus = components["schemas"]["UploadStatus"]

function makeFile(name: string, type: string, size: number): File {
  const file = new File([new Uint8Array(8)], name, { type })
  Object.defineProperty(file, "size", { value: size })
  return file
}

function statusDto(overrides: Partial<UploadStatus> = {}): UploadStatus {
  return {
    photoId: "photo-1",
    eventId: "event-1",
    status: "UPLOADING",
    uploadKind: "simple",
    filename: "a.jpg",
    mimeType: "image/jpeg",
    fileSize: 1024,
    errorMessage: null,
    createdAt: "2026-09-13T00:00:00Z",
    updatedAt: "2026-09-13T00:00:00Z",
    ...overrides,
  }
}

function simpleInit(overrides: Partial<InitializeUpload> = {}): InitializeUpload {
  return {
    photoId: "photo-1",
    uploadKind: "simple",
    uploadUrl: "https://storage.test/put",
    storageKey: "tenant/originals/a.jpg",
    expiresAt: "2026-09-13T00:10:00Z",
    partSize: null,
    ...overrides,
  }
}

function multipartInit(): InitializeUpload {
  return simpleInit({
    uploadKind: "multipart",
    uploadUrl: "https://storage.test/part/1",
    partSize: 10 * 1024 * 1024,
  })
}

function fakeRegistry(): UploadRegistry & { entries: UploadRegistryEntry[] } {
  const entries: UploadRegistryEntry[] = []
  return {
    entries,
    list: (eventId) => entries.filter((entry) => entry.eventId === eventId),
    upsert: (entry) => {
      const index = entries.findIndex((e) => e.key === entry.key)
      if (index >= 0) entries[index] = entry
      else entries.push(entry)
    },
    remove: (key) => {
      const index = entries.findIndex((e) => e.key === key)
      if (index >= 0) entries.splice(index, 1)
    },
  }
}

const okPut: PutTransport = async ({ onProgress }) => {
  onProgress?.(50, 100)
  return { etag: "etag-1" }
}

function makeDeps(overrides: Partial<UploadDeps> = {}) {
  let counter = 0
  const deps: UploadDeps = {
    initialize: vi.fn(async () => simpleInit()),
    rePresign: vi.fn(async () => ({
      uploadUrl: "https://storage.test/put-fresh",
      expiresAt: "2026-09-13T00:10:00Z",
    })),
    uploadStatus: vi.fn(async () => statusDto()),
    partUrls: vi.fn(async (_photoId: string, partNumbers: number[]) => ({
      partSize: 10 * 1024 * 1024,
      parts: partNumbers.map((partNumber) => ({
        partNumber,
        url: `https://storage.test/part/${partNumber}`,
      })),
    })),
    completeMultipart: vi.fn(async () => {}),
    abortMultipart: vi.fn(async () => {}),
    complete: vi.fn(async () => statusDto({ status: "PROCESSING" })),
    deletePhoto: vi.fn(async () => {}),
    put: vi.fn(okPut),
    concurrency: 3,
    maxAttempts: 3,
    sleep: vi.fn(async () => {}),
    randomUUID: () => `id-${(counter += 1)}`,
    registry: fakeRegistry(),
    ...overrides,
  }
  return deps
}

async function waitForTerminal(queue: UploadQueue): Promise<UploadItem> {
  await vi.waitFor(
    () => {
      const items = queue.getItems()
      expect(items).toHaveLength(1)
      expect(["processing", "failed", "interrupted"]).toContain(items[0].status)
    },
    { timeout: 2000 }
  )
  return queue.getItems()[0]
}

describe("UploadQueue", () => {
  it("rejects invalid files and queues valid ones", () => {
    const deps = makeDeps()
    const queue = new UploadQueue("event-1", deps)

    const rejected = queue.addFiles([
      makeFile("a.gif", "image/gif", 100),
      makeFile("b.jpg", "image/jpeg", 100),
    ])

    expect(rejected).toHaveLength(1)
    expect(rejected[0].filename).toBe("a.gif")
    expect(queue.getItems()).toHaveLength(1)
    expect(queue.getItems()[0].status).toBe("uploading")
  })

  it("uploads a simple file, completes it, and persists the registry", async () => {
    const deps = makeDeps()
    const queue = new UploadQueue("event-1", deps)
    queue.addFiles([makeFile("a.jpg", "image/jpeg", 1024)])

    const item = await waitForTerminal(queue)
    expect(item.status).toBe("processing")
    expect(item.photoId).toBe("photo-1")
    expect(deps.initialize).toHaveBeenCalledOnce()
    expect(deps.put).toHaveBeenCalledOnce()
    expect(deps.complete).toHaveBeenCalledOnce()
    expect(deps.registry?.list("event-1")).toHaveLength(1)
    expect(deps.registry?.list("event-1")[0].photoId).toBe("photo-1")
  })

  it("retries retryable transport errors with backoff", async () => {
    let calls = 0
    const put: PutTransport = async ({ onProgress }) => {
      calls += 1
      if (calls === 1) throw new UploadTransportError(500)
      onProgress?.(100, 100)
      return { etag: "etag-1" }
    }
    const deps = makeDeps({ put })
    const queue = new UploadQueue("event-1", deps)
    queue.addFiles([makeFile("a.jpg", "image/jpeg", 1024)])

    const item = await waitForTerminal(queue)
    expect(item.status).toBe("processing")
    expect(item.attempts).toBe(1)
    expect(deps.sleep).toHaveBeenCalledOnce()
  })

  it("fails immediately on non-retryable transport errors", async () => {
    const put: PutTransport = async () => {
      throw new UploadTransportError(400)
    }
    const deps = makeDeps({ put })
    const queue = new UploadQueue("event-1", deps)
    queue.addFiles([makeFile("a.jpg", "image/jpeg", 1024)])

    const item = await waitForTerminal(queue)
    expect(item.status).toBe("failed")
    expect(deps.sleep).not.toHaveBeenCalled()
  })

  it("chunks multipart uploads and completes with sorted parts", async () => {
    const completeMultipart = vi.fn(
      async (
        _photoId: string,
        _parts: { partNumber: number; etag: string }[]
      ) => {}
    )
    const deps = makeDeps({
      initialize: vi.fn(async () => multipartInit()),
      put: vi.fn(async () => ({ etag: "etag" })),
      completeMultipart,
    })
    const queue = new UploadQueue("event-1", deps)
    queue.addFiles([makeFile("big.jpg", "image/jpeg", 25 * 1024 * 1024)])

    const item = await waitForTerminal(queue)
    expect(item.status).toBe("processing")
    expect(deps.put).toHaveBeenCalledTimes(3)
    expect(completeMultipart).toHaveBeenCalledOnce()
    const parts = completeMultipart.mock.calls[0][1]
    expect(parts.map((part) => part.partNumber)).toEqual([1, 2, 3])
    expect(parts.every((part) => part.etag === "etag")).toBe(true)
  })

  it("cancels a multipart upload and deletes the photo", async () => {
    const hangingPut: PutTransport = ({ signal }) =>
      new Promise((_resolve, reject) => {
        signal?.addEventListener("abort", () =>
          reject(new DOMException("Aborted", "AbortError"))
        )
      })
    const deps = makeDeps({
      initialize: vi.fn(async () => multipartInit()),
      put: hangingPut,
    })
    const queue = new UploadQueue("event-1", deps)
    queue.addFiles([makeFile("big.jpg", "image/jpeg", 25 * 1024 * 1024)])

    await vi.waitFor(() => {
      expect(queue.getItems()[0]?.status).toBe("uploading")
    })
    await queue.cancel(queue.getItems()[0].key)

    expect(deps.abortMultipart).toHaveBeenCalledWith("photo-1")
    expect(deps.deletePhoto).toHaveBeenCalledWith("photo-1")
    expect(queue.getItems()).toHaveLength(0)
    expect(deps.registry?.list("event-1")).toHaveLength(0)
  })

  it("marks an interrupted registry entry for discard after reload", async () => {
    const registry = fakeRegistry()
    registry.entries.push({
      key: "id-1",
      eventId: "event-1",
      photoId: "photo-1",
      uploadKind: "simple",
      filename: "a.jpg",
      updatedAt: Date.now(),
    })
    const deps = makeDeps({
      registry,
      uploadStatus: vi.fn(async () => statusDto({ status: "UPLOADING" })),
    })
    const queue = new UploadQueue("event-1", deps)

    await queue.reconcile()

    const items = queue.getItems()
    expect(items).toHaveLength(1)
    expect(items[0].status).toBe("interrupted")
    expect(items[0].file).toBeNull()
  })

  it("resolves processing items from the authoritative photo list", async () => {
    const deps = makeDeps()
    const queue = new UploadQueue("event-1", deps)
    queue.addFiles([makeFile("a.jpg", "image/jpeg", 1024)])
    await waitForTerminal(queue)

    queue.applyPhotoStatuses([
      { id: "photo-1", status: "READY", errorMessage: null },
    ])
    expect(queue.getItems()[0].status).toBe("ready")
  })

  it("marks processing items failed when the worker reports failure", async () => {
    const deps = makeDeps()
    const queue = new UploadQueue("event-1", deps)
    queue.addFiles([makeFile("a.jpg", "image/jpeg", 1024)])
    await waitForTerminal(queue)

    queue.applyPhotoStatuses([
      { id: "photo-1", status: "FAILED", errorMessage: "boom" },
    ])
    expect(queue.getItems()[0].status).toBe("failed")
    expect(queue.getItems()[0].error).toBe("boom")
  })

  it("drops registry entries for photos that finished processing", async () => {
    const registry = fakeRegistry()
    registry.entries.push({
      key: "id-1",
      eventId: "event-1",
      photoId: "photo-1",
      uploadKind: "simple",
      filename: "a.jpg",
      updatedAt: Date.now(),
    })
    const deps = makeDeps({
      registry,
      uploadStatus: vi.fn(async () => statusDto({ status: "READY" })),
    })
    const queue = new UploadQueue("event-1", deps)

    await queue.reconcile()

    expect(queue.getItems()).toHaveLength(0)
    expect(registry.entries).toHaveLength(0)
  })
})
