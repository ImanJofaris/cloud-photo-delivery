export type CookieSink = {
  read: () => Promise<string | null> | string | null
  write: (token: string) => Promise<void> | void
  clear: () => Promise<void> | void
}

export type GoFetch = (path: string, init?: RequestInit) => Promise<Response>

type Envelope = {
  data?: unknown
  error?: { code?: string; message?: string } | null
}

type AuthData = {
  accessToken?: string
  refreshToken?: string
  expiresIn?: number
  user?: unknown
}

export function makeGoFetch(
  baseUrl: string,
  fetchImpl: typeof fetch = fetch
): GoFetch {
  const base = baseUrl.replace(/\/$/, "")
  return (path, init) => fetchImpl(`${base}${path}`, init)
}

function json(body: unknown, status: number) {
  return Response.json(body, { status })
}

export function serviceUnavailable() {
  return json(
    {
      data: null,
      error: {
        code: "SERVICE_UNAVAILABLE",
        message: "The API is not configured on this server.",
      },
    },
    503
  )
}

function envelopeError(code: string, message: string, status: number) {
  return json({ data: null, error: { code, message } }, status)
}

async function readEnvelope(response: Response): Promise<Envelope | null> {
  return (await response.json().catch(() => null)) as Envelope | null
}

function postJson(goFetch: GoFetch, path: string, body: unknown) {
  return goFetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })
}

async function authenticate(
  request: Request,
  goFetch: GoFetch,
  cookie: CookieSink,
  path: string
): Promise<Response> {
  let body: unknown
  try {
    body = await request.json()
  } catch {
    return envelopeError("VALIDATION_ERROR", "Invalid JSON body", 422)
  }

  const response = await postJson(goFetch, path, body)
  const payload = await readEnvelope(response)

  if (!response.ok) {
    return json(
      payload ?? {
        data: null,
        error: { code: "UNKNOWN", message: "Request failed" },
      },
      response.status
    )
  }

  const data = (payload?.data ?? {}) as AuthData
  if (!data.accessToken || !data.refreshToken) {
    return envelopeError("UNKNOWN", "Malformed auth response", 502)
  }

  await cookie.write(data.refreshToken)

  return json(
    {
      data: {
        accessToken: data.accessToken,
        expiresIn: data.expiresIn,
        user: data.user,
      },
      error: null,
    },
    200
  )
}

export function loginHandler(
  request: Request,
  goFetch: GoFetch,
  cookie: CookieSink
): Promise<Response> {
  return authenticate(request, goFetch, cookie, "/api/v1/auth/login")
}

export function signupHandler(
  request: Request,
  goFetch: GoFetch,
  cookie: CookieSink
): Promise<Response> {
  return authenticate(request, goFetch, cookie, "/api/v1/auth/signup")
}

const inflightRefreshes = new Map<
  string,
  Promise<{ status: number; payload: Envelope | null }>
>()

function refreshOnce(goFetch: GoFetch, token: string) {
  const existing = inflightRefreshes.get(token)
  if (existing) {
    return existing
  }

  const promise = (async () => {
    try {
      const response = await postJson(goFetch, "/api/v1/auth/refresh", {
        refreshToken: token,
      })
      return { status: response.status, payload: await readEnvelope(response) }
    } catch {
      return { status: 502, payload: null }
    } finally {
      inflightRefreshes.delete(token)
    }
  })()

  inflightRefreshes.set(token, promise)
  return promise
}

export async function refreshHandler(
  _request: Request,
  goFetch: GoFetch,
  cookie: CookieSink
): Promise<Response> {
  const token = await cookie.read()
  if (!token) {
    return envelopeError("UNAUTHENTICATED", "No active session", 401)
  }

  const { status, payload } = await refreshOnce(goFetch, token)
  const data = (payload?.data ?? {}) as AuthData

  if (status >= 400 || !data.accessToken || !data.refreshToken) {
    await cookie.clear()
    return json(
      payload ?? {
        data: null,
        error: { code: "UNAUTHENTICATED", message: "Session expired" },
      },
      status >= 400 ? status : 401
    )
  }

  await cookie.write(data.refreshToken)

  return json(
    {
      data: {
        accessToken: data.accessToken,
        expiresIn: data.expiresIn,
        user: data.user,
      },
      error: null,
    },
    200
  )
}

export async function logoutHandler(
  _request: Request,
  goFetch: GoFetch,
  cookie: CookieSink
): Promise<Response> {
  const token = await cookie.read()
  if (token) {
    try {
      await postJson(goFetch, "/api/v1/auth/logout", { refreshToken: token })
    } catch {
      // The cookie is cleared regardless; the refresh token expires server-side.
    }
  }

  await cookie.clear()
  return json({ data: { ok: true }, error: null }, 200)
}
