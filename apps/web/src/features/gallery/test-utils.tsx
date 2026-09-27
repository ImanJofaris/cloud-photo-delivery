import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import * as React from "react"

export function createQueryWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0 },
      mutations: { retry: false },
    },
  })

  return function QueryWrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
  }
}

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  })
}

export function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === "string") return input
  if (input instanceof URL) return input.toString()
  return input.url
}

export async function requestBody<T>(
  input: RequestInfo | URL,
  init?: RequestInit
): Promise<T> {
  if (typeof init?.body === "string") return JSON.parse(init.body) as T
  if (input instanceof Request) return (await input.clone().json()) as T
  return {} as T
}

export function batchUrlResponse(
  variant: string,
  photoIds: string[]
): Response {
  const urls = Object.fromEntries(
    photoIds.map((photoId) => [photoId, `https://r2.test/${variant}.jpg`])
  )
  return jsonResponse({ data: { urls, expiresIn: 300 }, error: null })
}
