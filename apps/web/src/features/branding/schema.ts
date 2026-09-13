import { z } from "zod"

import type { components } from "@workspace/api-client"

export const BRANDING_ASSET_MIME_TYPES = [
  "image/png",
  "image/jpeg",
  "image/webp",
] as const

export const MAX_BRANDING_ASSET_BYTES = 5 * 1024 * 1024

export const MAX_BUSINESS_NAME_LENGTH = 255

export type BrandingAssetKind = "logo" | "profileImage"

export type BrandingAssetFile = {
  name: string
  type: string
  size: number
}

export function validateBrandingAsset(file: BrandingAssetFile): string | null {
  if (
    !BRANDING_ASSET_MIME_TYPES.includes(
      file.type as (typeof BRANDING_ASSET_MIME_TYPES)[number]
    )
  ) {
    return "Use a PNG, JPEG, or WebP image."
  }
  if (file.size < 1) {
    return "The image is empty."
  }
  if (file.size > MAX_BRANDING_ASSET_BYTES) {
    return "Images must be 5 MB or smaller."
  }
  return null
}

const hexColor = z
  .string()
  .trim()
  .transform((value) => value.toLowerCase())
  .refine(
    (value) => value === "" || /^#[0-9a-f]{6}$/.test(value),
    "Use a lowercase hex color like #1a2b3c."
  )

export const brandingSchema = z.object({
  businessName: z
    .string()
    .trim()
    .max(
      MAX_BUSINESS_NAME_LENGTH,
      `Business name must be ${MAX_BUSINESS_NAME_LENGTH} characters or fewer.`
    ),
  primaryColor: hexColor,
  secondaryColor: hexColor,
  contactEmail: z
    .union([z.literal(""), z.email("Enter a valid email address.")])
    .transform((value) => value.trim()),
  contactPhone: z.string().trim().max(40, "Phone must be 40 characters or fewer."),
  websiteUrl: z
    .union([
      z.literal(""),
      z.url({ protocol: /^https?$/, error: "Enter a valid http(s) URL." }),
    ])
    .transform((value) => value.trim()),
})

export type BrandingValues = z.infer<typeof brandingSchema>

type BrandingResponse = components["schemas"]["Branding"]
type BrandingUpdateRequest = components["schemas"]["BrandingUpdateRequest"]

const TEXT_FIELDS = [
  "businessName",
  "primaryColor",
  "secondaryColor",
  "contactEmail",
  "contactPhone",
  "websiteUrl",
] as const

function comparable(
  field: (typeof TEXT_FIELDS)[number],
  value: string | null | undefined
): string {
  const trimmed = (value ?? "").trim()
  return field === "primaryColor" || field === "secondaryColor"
    ? trimmed.toLowerCase()
    : trimmed
}

export function toBrandingPatch({
  initial,
  values,
  logoKey,
  profileImageKey,
}: {
  initial: BrandingResponse | undefined
  values: BrandingValues
  logoKey: string | null
  profileImageKey: string | null
}): BrandingUpdateRequest {
  const patch: BrandingUpdateRequest = {}

  for (const field of TEXT_FIELDS) {
    const next = comparable(field, values[field])
    const previous = comparable(field, initial?.[field])
    if (next !== previous) {
      patch[field] = next
    }
  }

  if (logoKey !== null) patch.logoKey = logoKey
  if (profileImageKey !== null) patch.profileImageKey = profileImageKey

  return patch
}

export function brandingValuesFrom(
  branding: BrandingResponse | undefined
): BrandingValues {
  return {
    businessName: branding?.businessName ?? "",
    primaryColor: (branding?.primaryColor ?? "").toLowerCase(),
    secondaryColor: (branding?.secondaryColor ?? "").toLowerCase(),
    contactEmail: branding?.contactEmail ?? "",
    contactPhone: branding?.contactPhone ?? "",
    websiteUrl: branding?.websiteUrl ?? "",
  }
}
