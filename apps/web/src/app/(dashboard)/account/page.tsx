"use client"

import { useQuery } from "@tanstack/react-query"

import type { components } from "@workspace/api-client"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { apiCall } from "@/lib/auth/api"
import { useSession } from "@/lib/auth/session-provider"

type Profile = components["schemas"]["Profile"]

export default function AccountPage() {
  const { user } = useSession()
  const { data, isPending, error } = useQuery({
    queryKey: ["account", "me"],
    queryFn: () => apiCall<Profile>((client) => client.GET("/account/me")),
  })

  return (
    <div className="max-w-xl space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Account</h1>
        <p className="text-sm text-muted-foreground">Your operator profile.</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Profile</CardTitle>
          <CardDescription>
            Read from the Go API with your access token.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {isPending && <Skeleton className="h-16 w-full" />}

          {error && (
            <p className="text-destructive">
              Could not load your profile. Please refresh the page.
            </p>
          )}

          {data && (
            <dl className="grid gap-2">
              <div className="flex justify-between gap-4">
                <dt className="text-muted-foreground">Email</dt>
                <dd>{data.email}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-muted-foreground">Business name</dt>
                <dd>{data.businessName || "Not set"}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-muted-foreground">Account ID</dt>
                <dd className="font-mono text-xs">{data.id}</dd>
              </div>
            </dl>
          )}

          {!data && !isPending && user && (
            <p className="text-muted-foreground">{user.email}</p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
