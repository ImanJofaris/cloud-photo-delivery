"use client"

import * as React from "react"
import { useRouter } from "next/navigation"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { useSession } from "@/lib/auth/session-provider"

import { AdminHealthCard } from "./admin-health-card"
import { AdminStatsGrid } from "./admin-stats"
import { AdminSubscriptionsTable } from "./admin-subscriptions-table"
import { AdminUsersTable } from "./admin-users-table"
import {
  useAdminHealth,
  useAdminStats,
  useAdminSubscriptions,
  useAdminUsers,
} from "./api"
import { adminErrorMessage } from "./errors"

function AdminHeader() {
  return (
    <div>
      <h1 className="text-2xl font-semibold">Admin</h1>
      <p className="text-sm text-muted-foreground">Read-only overview.</p>
    </div>
  )
}

function SectionError({ error }: { error: unknown }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Could not load this section</CardTitle>
        <CardDescription>Refresh the page to try again.</CardDescription>
      </CardHeader>
      <CardContent className="text-sm text-destructive">
        {adminErrorMessage(error)}
      </CardContent>
    </Card>
  )
}

export function AdminPanel() {
  const { status, user } = useSession()
  const router = useRouter()
  const isAdmin = Boolean(user?.isAdmin)
  const enabled = status === "authenticated" && isAdmin

  React.useEffect(() => {
    if (status === "authenticated" && !isAdmin) {
      router.replace("/dashboard")
    }
  }, [status, isAdmin, router])

  const statsQuery = useAdminStats(enabled)
  const healthQuery = useAdminHealth(enabled)
  const usersQuery = useAdminUsers(enabled)
  const subscriptionsQuery = useAdminSubscriptions(enabled)

  const users = usersQuery.data?.pages.flatMap((page) => page.users) ?? []
  const subscriptions =
    subscriptionsQuery.data?.pages.flatMap((page) => page.subscriptions) ?? []

  if (!enabled) {
    return (
      <div className="space-y-6" aria-busy="true">
        <AdminHeader />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-56 w-full" />
        <Skeleton className="h-64 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <AdminHeader />

      {statsQuery.isError ? (
        <SectionError error={statsQuery.error} />
      ) : (
        <AdminStatsGrid
          stats={statsQuery.data}
          loading={statsQuery.isPending}
        />
      )}

      {healthQuery.isError ? (
        <SectionError error={healthQuery.error} />
      ) : (
        <AdminHealthCard
          health={healthQuery.data}
          loading={healthQuery.isPending}
        />
      )}

      {usersQuery.isError ? (
        <SectionError error={usersQuery.error} />
      ) : usersQuery.isPending ? (
        <Skeleton className="h-64 w-full" />
      ) : (
        <AdminUsersTable
          users={users}
          hasNextPage={Boolean(usersQuery.hasNextPage)}
          isFetchingNextPage={usersQuery.isFetchingNextPage}
          onLoadMore={() => void usersQuery.fetchNextPage()}
        />
      )}

      {subscriptionsQuery.isError ? (
        <SectionError error={subscriptionsQuery.error} />
      ) : subscriptionsQuery.isPending ? (
        <Skeleton className="h-64 w-full" />
      ) : (
        <AdminSubscriptionsTable
          subscriptions={subscriptions}
          hasNextPage={Boolean(subscriptionsQuery.hasNextPage)}
          isFetchingNextPage={subscriptionsQuery.isFetchingNextPage}
          onLoadMore={() => void subscriptionsQuery.fetchNextPage()}
        />
      )}
    </div>
  )
}
