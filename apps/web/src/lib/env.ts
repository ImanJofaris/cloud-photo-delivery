export const API_VERSION_PATH = "/api/v1"

export function serverApiBaseUrl(): string {
  const value = process.env.API_BASE_URL
  if (!value) {
    throw new Error("API_BASE_URL is not set")
  }
  return value.replace(/\/$/, "")
}

export function publicApiBaseUrl(): string {
  const value = process.env.NEXT_PUBLIC_API_BASE_URL
  if (!value) {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set")
  }
  return value.replace(/\/$/, "")
}

export function apiVersionedBaseUrl(): string {
  const origin = publicApiBaseUrl()
  return origin.endsWith(API_VERSION_PATH)
    ? origin
    : `${origin}${API_VERSION_PATH}`
}
