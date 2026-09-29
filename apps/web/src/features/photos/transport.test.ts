import { afterEach, describe, expect, it, vi } from "vitest"

import { putWithProgress } from "./transport"

class FakeXHR {
  static instances: FakeXHR[] = []

  timeout = 0
  status = 0
  upload = { onprogress: null as ((event: ProgressEvent) => void) | null }
  onload: (() => void) | null = null
  onerror: (() => void) | null = null
  ontimeout: (() => void) | null = null
  onabort: (() => void) | null = null

  constructor() {
    FakeXHR.instances.push(this)
  }

  open() {}
  setRequestHeader() {}
  getResponseHeader() {
    return null
  }
  send() {}
  abort() {}
}

afterEach(() => {
  FakeXHR.instances = []
  vi.unstubAllGlobals()
})

describe("putWithProgress", () => {
  it("sets the default timeout and rejects timeouts as retryable transport errors", async () => {
    vi.stubGlobal("XMLHttpRequest", FakeXHR)
    const promise = putWithProgress({
      url: "https://r2.test/put",
      body: new Blob(["x"]),
    })

    const xhr = FakeXHR.instances[0]
    expect(xhr.timeout).toBe(15 * 60 * 1000)

    xhr.ontimeout?.()
    await expect(promise).rejects.toMatchObject({ status: 0 })
  })

  it("honors a custom timeout", async () => {
    vi.stubGlobal("XMLHttpRequest", FakeXHR)
    const promise = putWithProgress({
      url: "https://r2.test/put",
      body: new Blob(["x"]),
      timeoutMs: 1000,
    })

    expect(FakeXHR.instances[0].timeout).toBe(1000)

    FakeXHR.instances[0].ontimeout?.()
    await expect(promise).rejects.toThrow("The upload timed out.")
  })
})
