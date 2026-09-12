import { Button } from "@workspace/ui/components/button"

import { ApiStatus } from "@/components/api-status"

export const dynamic = "force-dynamic"

const apiBaseUrl = process.env.API_BASE_URL

async function getApiStatus(): Promise<"online" | "offline" | "unconfigured"> {
  if (!apiBaseUrl) {
    return "unconfigured"
  }

  try {
    const response = await fetch(`${apiBaseUrl}/readyz`, { cache: "no-store" })
    return response.ok ? "online" : "offline"
  } catch {
    return "offline"
  }
}

export default async function Page() {
  const status = await getApiStatus()

  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-6 p-6">
      <div className="flex max-w-md flex-col items-center gap-2 text-center">
        <h1 className="text-2xl font-semibold">Cloud Photo Delivery</h1>
        <p className="text-sm text-muted-foreground">
          Operator dashboard foundation is ready.
        </p>
      </div>
      <ApiStatus status={status} />
      <Button>Get started</Button>
    </main>
  )
}
