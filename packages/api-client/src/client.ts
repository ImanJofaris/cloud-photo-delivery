import createClient, { type Middleware } from "openapi-fetch"

import type { paths } from "./schema"

export type ApiErrorBody = {
  code: string
  message: string
}

export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(code: string, message: string, status: number) {
    super(message)
    this.name = "ApiError"
    this.code = code
    this.status = status
  }
}

export type TokenProvider = () => string | null | undefined

export interface ApiClientOptions {
  baseUrl: string
  getToken?: TokenProvider
}

export type EnvelopeResult<T> = {
  data: T
  error: null
}

type RawEnvelope = {
  data?: unknown
  error?: ApiErrorBody | null
}

type ClientResult = {
  data?: unknown
  error?: unknown
  response: Response
}

function normalizeError(error: unknown): ApiErrorBody {
  if (error && typeof error === "object") {
    const body = error as Partial<ApiErrorBody>
    return {
      code: typeof body.code === "string" ? body.code : "UNKNOWN",
      message:
        typeof body.message === "string" ? body.message : "Request failed",
    }
  }
  return { code: "UNKNOWN", message: "Request failed" }
}

export function unwrapEnvelope<T>(result: ClientResult): T {
  const { response } = result
  const body = result.data as RawEnvelope | undefined

  if (result.error !== undefined) {
    const { code, message } = normalizeError(result.error)
    throw new ApiError(code, message, response.status)
  }

  if (body?.error) {
    throw new ApiError(body.error.code, body.error.message, response.status)
  }

  if (!response.ok) {
    throw new ApiError("UNKNOWN", "Request failed", response.status)
  }

  return body?.data as T
}

export async function unwrap<T>(result: Promise<ClientResult>): Promise<T> {
  return unwrapEnvelope<T>(await result)
}

export function createApiClient({ baseUrl, getToken }: ApiClientOptions) {
  const client = createClient<paths>({ baseUrl })

  if (getToken) {
    const auth: Middleware = {
      onRequest({ request }) {
        const token = getToken()
        if (token) {
          request.headers.set("Authorization", `Bearer ${token}`)
        }
        return request
      },
    }
    client.use(auth)
  }

  return client
}
