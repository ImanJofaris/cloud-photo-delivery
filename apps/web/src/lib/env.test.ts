import { afterEach, describe, expect, it } from "vitest"

import { API_VERSION_PATH, apiVersionedBaseUrl, publicApiBaseUrl } from "./env"

const ORIGINAL = process.env.NEXT_PUBLIC_API_BASE_URL

afterEach(() => {
  if (ORIGINAL === undefined) {
    delete process.env.NEXT_PUBLIC_API_BASE_URL
  } else {
    process.env.NEXT_PUBLIC_API_BASE_URL = ORIGINAL
  }
})

describe("publicApiBaseUrl", () => {
  it("returns the origin without a trailing slash", () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080/"
    expect(publicApiBaseUrl()).toBe("http://localhost:18080")
  })

  it("throws when the env var is missing", () => {
    delete process.env.NEXT_PUBLIC_API_BASE_URL
    expect(() => publicApiBaseUrl()).toThrow("NEXT_PUBLIC_API_BASE_URL")
  })
})

describe("apiVersionedBaseUrl", () => {
  it("appends the API version path to an origin", () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080"
    expect(apiVersionedBaseUrl()).toBe(
      `http://localhost:18080${API_VERSION_PATH}`
    )
  })

  it("normalizes a trailing slash before appending", () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080/"
    expect(apiVersionedBaseUrl()).toBe(
      `http://localhost:18080${API_VERSION_PATH}`
    )
  })

  it("does not double the version path", () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:18080/api/v1"
    expect(apiVersionedBaseUrl()).toBe("http://localhost:18080/api/v1")
  })
})
