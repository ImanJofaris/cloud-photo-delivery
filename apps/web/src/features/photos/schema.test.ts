import { describe, expect, it } from "vitest"

import { MAX_FILE_SIZE, uploadKindFor, validateFile } from "./schema"

describe("validateFile", () => {
  it("accepts supported image types", () => {
    expect(
      validateFile({ name: "a.jpg", type: "image/jpeg", size: 1024 })
    ).toBeNull()
    expect(
      validateFile({ name: "a.png", type: "image/png", size: 1024 })
    ).toBeNull()
    expect(
      validateFile({ name: "a.webp", type: "image/webp", size: 1024 })
    ).toBeNull()
  })

  it("rejects unsupported types", () => {
    expect(
      validateFile({ name: "a.gif", type: "image/gif", size: 1024 })
    ).toMatch(/JPEG/)
  })

  it("rejects empty and oversized files", () => {
    expect(
      validateFile({ name: "a.jpg", type: "image/jpeg", size: 0 })
    ).toMatch(/empty/)
    expect(
      validateFile({
        name: "a.jpg",
        type: "image/jpeg",
        size: MAX_FILE_SIZE + 1,
      })
    ).toMatch(/100 MB/)
  })

  it("rejects blank filenames", () => {
    expect(
      validateFile({ name: "  ", type: "image/jpeg", size: 1024 })
    ).toMatch(/name/)
  })
})

describe("uploadKindFor", () => {
  it("uses simple uploads below 10 MB and multipart at or above", () => {
    expect(uploadKindFor(5 * 1024 * 1024)).toBe("simple")
    expect(uploadKindFor(10 * 1024 * 1024)).toBe("multipart")
  })
})
