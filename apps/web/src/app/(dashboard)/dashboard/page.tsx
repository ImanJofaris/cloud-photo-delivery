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

import { useSession } from "@/lib/auth/session-provider"

export default function DashboardPage() {
  const { status, user } = useSession()

  if (status === "loading") {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-32 w-full max-w-xl" />
      </div>
    )
  }

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
          <CardTitle>Events</CardTitle>
          <CardDescription>
            Event management arrives in Phase F2. The API is already live.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            render={<Link href="/account" />}
            nativeButton={false}
            variant="outline"
          >
            View account
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
