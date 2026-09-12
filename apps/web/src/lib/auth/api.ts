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

export async function apiCall<T>(
  call: (client: Client) => Promise<ClientResult>
): Promise<T> {
  try {
    return unwrapEnvelope<T>(await call(getClient()))
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      const refreshed = await refreshSession()
      if (refreshed) {
        return unwrapEnvelope<T>(await call(getClient()))
      }
      clearSession()
    }
    throw error
  }
}
