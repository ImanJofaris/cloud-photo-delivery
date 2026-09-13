import { describe, expect, it, vi } from "vitest"

import { UploadTransportError } from "@/features/photos/transport"

import { uploadBrandingAsset } from "./asset-upload"

function pngFile(): File {
  return new File([new Uint8Array(8)], "logo.png", { type: "image/png" })
}

function asset(storageKey: string) {
  return {
    uploadUrl: `https://r2.test/${storageKey}`,
    storageKey,
    expiresAt: "2026-09-13T10:05:00Z",
  }
}

describe("uploadBrandingAsset", () => {
  it("presigns, uploads with progress, and returns the key in order", async () => {
    const order: string[] = []
    const presign = vi.fn(async () => {
      order.push("presign")
      return asset("tenant/u/branding/logo/a.png")
    })
    const put = vi.fn(
      async ({ onProgress }: { onProgress?: (loaded: number, total: number) => void }) => {
        order.push("put")
        onProgress?.(5, 10)
        return { etag: "etag-1" }
      }
    )
    const progress: number[] = []

    const result = await uploadBrandingAsset({
      file: pngFile(),
      kind: "logo",
      presign,
      put,
      onProgress: (percent) => progress.push(percent),
    })

    expect(order).toEqual(["presign", "put"])
    expect(result.storageKey).toBe("tenant/u/branding/logo/a.png")
    expect(progress).toEqual([50])
  })

  it("requests a fresh URL when the presigned PUT has expired", async () => {
    const presign = vi
      .fn()
      .mockResolvedValueOnce(asset("tenant/u/branding/logo/a.png"))
      .mockResolvedValueOnce(asset("tenant/u/branding/logo/b.png"))
    const put = vi
      .fn()
      .mockRejectedValueOnce(new UploadTransportError(403))
      .mockResolvedValueOnce({ etag: "etag-2" })

    const result = await uploadBrandingAsset({
      file: pngFile(),
      kind: "logo",
      presign,
      put,
    })

    expect(presign).toHaveBeenCalledTimes(2)
    expect(put).toHaveBeenCalledTimes(2)
    expect(result.storageKey).toBe("tenant/u/branding/logo/b.png")
  })

  it("rethrows non-expiry failures without a second presign", async () => {
    const presign = vi
      .fn()
      .mockResolvedValue(asset("tenant/u/branding/logo/a.png"))
    const put = vi.fn().mockRejectedValue(new UploadTransportError(500))

    await expect(
      uploadBrandingAsset({
        file: pngFile(),
        kind: "logo",
        presign,
        put,
      })
    ).rejects.toBeInstanceOf(UploadTransportError)
    expect(presign).toHaveBeenCalledTimes(1)
  })
})
