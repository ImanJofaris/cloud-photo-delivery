export type SessionUser = {
  id: string
  email: string
  businessName?: string
  isAdmin?: boolean
}

export type SessionStatus = "loading" | "authenticated" | "anonymous"

export type SessionState = {
  status: SessionStatus
  user: SessionUser | null
  accessToken: string | null
}

const INITIAL_STATE: SessionState = {
  status: "loading",
  user: null,
  accessToken: null,
}

let state: SessionState = INITIAL_STATE

const listeners = new Set<() => void>()

function setState(next: SessionState) {
  state = next
  listeners.forEach((listener) => listener())
}

export function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function getSnapshot(): SessionState {
  return state
}

export function getServerSnapshot(): SessionState {
  return INITIAL_STATE
}

export function getAccessToken(): string | null {
  return state.accessToken
}

export function setSession(data: {
  accessToken: string
  user?: SessionUser | null
}) {
  setState({
    status: "authenticated",
    accessToken: data.accessToken,
    user: data.user ?? null,
  })
}

export function clearSession() {
  setState({ status: "anonymous", user: null, accessToken: null })
}

let refreshPromise: Promise<boolean> | null = null

async function performRefresh(): Promise<boolean> {
  try {
    const response = await fetch("/api/auth/refresh", { method: "POST" })
    const payload = (await response.json().catch(() => null)) as {
      data?: { accessToken?: string; user?: SessionUser }
    } | null

    if (!response.ok || !payload?.data?.accessToken) {
      clearSession()
      return false
    }

    setSession({
      accessToken: payload.data.accessToken,
      user: payload.data.user ?? null,
    })
    return true
  } catch {
    clearSession()
    return false
  }
}

function withRefreshLock<T>(fn: () => Promise<T>): Promise<T> {
  if (typeof navigator !== "undefined" && "locks" in navigator) {
    return navigator.locks.request("cpd-refresh", fn) as Promise<T>
  }
  return fn()
}

export function refreshSession(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = withRefreshLock(performRefresh).finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}

export function resetSessionForTests() {
  state = INITIAL_STATE
  refreshPromise = null
  listeners.clear()
}
