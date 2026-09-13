"use client"

import Link from "next/link"

import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { useSubscription } from "@/features/billing/api"
import { formatByteLimit, formatLimit } from "@/features/billing/format"
import { formatBytes } from "@/features/events/format"
import { useSession } from "@/lib/auth/session-provider"

export default function DashboardPage() {
  const { status, user } = useSession()
  const subscriptionQuery = useSubscription()

  if (status === "loading") {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-32 w-full max-w-xl" />
      </div>
    )
  }

  const view = subscriptionQuery.data

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Dashboard</h1>
        <p className="text-sm text-muted-foreground">
          {user?.businessName
            ? `Welcome back, ${user.businessName}.`
            : "Welcome back."}
        </p>
      </div>

      <Card className="max-w-xl">
        <CardHeader>
          <CardTitle>{view ? `${view.plan.name} plan` : "Your plan"}</CardTitle>
          <CardDescription>
            Usage against your current plan limits.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {subscriptionQuery.isPending && (
            <Skeleton className="h-16 w-full" />
          )}

          {subscriptionQuery.isError && (
            <p className="text-sm text-muted-foreground">
              Could not load your plan right now.
            </p>
          )}

          {view && (
            <dl className="grid gap-2 text-sm">
              <div className="flex justify-between gap-4">
                <dt className="text-muted-foreground">Active events</dt>
                <dd>
                  {view.usage.activeEvents} of{" "}
                  {formatLimit(view.plan.limits.activeEvents)}
                </dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-muted-foreground">Storage</dt>
                <dd>
                  {formatBytes(view.usage.storageBytes)} of{" "}
                  {formatByteLimit(view.plan.limits.storageBytes)}
                </dd>
              </div>
            </dl>
          )}

          <Button
            render={<Link href="/billing" />}
            nativeButton={false}
            variant="outline"
          >
            Manage billing
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
