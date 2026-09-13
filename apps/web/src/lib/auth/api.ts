import {
  ApiError,
  createApiClient,
  unwrapEnvelope,
} from "@workspace/api-client"

import { apiVersionedBaseUrl } from "@/lib/env"

import { clearSession, getAccessToken, refreshSession } from "./session"

type Client = ReturnType<typeof createApiClient>
type ClientResult = { data?: unknown; error?: unknown; response: Response }

let client: Client | null = null

function getClient(): Client {
  if (!client) {
    client = createApiClient({
      baseUrl: apiVersionedBaseUrl(),
      getToken: getAccessToken,
    })
  }
  return client
}

export function resetApiClientForTests() {
  client = null
}

async function withRefreshRetry<T>(
  call: (client: Client) => Promise<ClientResult>,
  unwrap: (result: ClientResult) => T
): Promise<T> {
  try {
    return unwrap(await call(getClient()))
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      const refreshed = await refreshSession()
      if (refreshed) {
        return unwrap(await call(getClient()))
      }
      clearSession()
    }
    throw error
  }
}

export function apiCall<T>(
  call: (client: Client) => Promise<ClientResult>
): Promise<T> {
  return withRefreshRetry(call, (result) => unwrapEnvelope<T>(result))
}

function errorFromResult(result: ClientResult): ApiError {
  const raw = result.error
  const body =
    raw && typeof raw === "object" && "error" in raw
      ? (raw as { error?: { code?: string; message?: string } | null }).error
      : null
  return new ApiError(
    typeof body?.code === "string" ? body.code : "UNKNOWN",
    typeof body?.message === "string" ? body.message : "Request failed",
    result.response.status
  )
}

export function apiBlobCall(
  call: (client: Client) => Promise<ClientResult>
): Promise<Blob> {
  return withRefreshRetry(call, (result) => {
    if (result.response.ok && result.data) {
      return result.data as Blob
    }
    throw errorFromResult(result)
  })
}
