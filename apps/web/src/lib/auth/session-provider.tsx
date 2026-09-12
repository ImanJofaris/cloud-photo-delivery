"use client"

import * as React from "react"

import { ApiError } from "@workspace/api-client"

import {
  clearSession,
  getServerSnapshot,
  getSnapshot,
  refreshSession,
  setSession,
  subscribe,
  type SessionState,
  type SessionUser,
} from "./session"

export type AuthInput = {
  email: string
  password: string
  businessName?: string
}

type AuthResult = {
  accessToken: string
  user?: SessionUser
}

type SessionContextValue = SessionState & {
  login: (input: AuthInput) => Promise<AuthResult>
  signup: (input: AuthInput) => Promise<AuthResult>
  logout: () => Promise<void>
}

const SessionContext = React.createContext<SessionContextValue | null>(null)

async function postAuth(path: string, input: AuthInput): Promise<AuthResult> {
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  })
  const payload = (await response.json().catch(() => null)) as {
    data?: AuthResult
    error?: { code?: string; message?: string } | null
  } | null

  if (!response.ok || payload?.error || !payload?.data?.accessToken) {
    throw new ApiError(
      payload?.error?.code ?? "UNKNOWN",
      payload?.error?.message ?? "Request failed",
      response.status
    )
  }

  return payload.data
}

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const state = React.useSyncExternalStore(
    subscribe,
    getSnapshot,
    getServerSnapshot
  )

  React.useEffect(() => {
    void refreshSession()
  }, [])

  const login = React.useCallback(async (input: AuthInput) => {
    const data = await postAuth("/api/auth/login", input)
    setSession(data)
    return data
  }, [])

  const signup = React.useCallback(async (input: AuthInput) => {
    const data = await postAuth("/api/auth/signup", input)
    setSession(data)
    return data
  }, [])

  const logout = React.useCallback(async () => {
    try {
      await fetch("/api/auth/logout", { method: "POST" })
    } finally {
      clearSession()
    }
  }, [])

  const value = React.useMemo<SessionContextValue>(
    () => ({ ...state, login, signup, logout }),
    [state, login, signup, logout]
  )

  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  )
}

export function useSession(): SessionContextValue {
  const context = React.useContext(SessionContext)
  if (!context) {
    throw new Error("useSession must be used within a SessionProvider")
  }
  return context
}
