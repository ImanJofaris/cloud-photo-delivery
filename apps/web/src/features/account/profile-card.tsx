"use client"

import type { components } from "@workspace/api-client"

import { Avatar, AvatarFallback } from "@workspace/ui/components/avatar"
import { Badge } from "@workspace/ui/components/badge"
import { Card, CardContent } from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { initials, roleLabel } from "./format"

type Profile = components["schemas"]["Profile"]

function DetailRow({
  label,
  value,
  mono = false,
}: {
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="flex items-center justify-between gap-4 py-2.5">
      <dt className="text-muted-foreground">{label}</dt>
      <dd
        className={
          mono ? "truncate font-mono text-xs" : "truncate font-medium"
        }
      >
        {value}
      </dd>
    </div>
  )
}

export function ProfileCard({
  profile,
  loading,
  error,
  planName,
  planLoading,
}: {
  profile?: Profile
  loading: boolean
  error: boolean
  planName?: string
  planLoading: boolean
}) {
  return (
    <Card>
      <CardContent className="space-y-4">
        {loading && (
          <>
            <div className="flex items-center gap-3.5">
              <Skeleton className="size-12 rounded-2xl" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-4 w-52" />
              </div>
            </div>
            <Skeleton className="h-36 w-full" />
          </>
        )}

        {error && (
          <p className="text-sm text-destructive">
            Could not load your profile. Please refresh the page.
          </p>
        )}

        {profile && (
          <>
            <div className="flex items-center gap-3.5">
              <Avatar className="size-12 rounded-2xl">
                <AvatarFallback className="rounded-2xl bg-muted text-sm font-semibold">
                  {initials(profile.businessName || profile.email) || "OP"}
                </AvatarFallback>
              </Avatar>
              <div className="min-w-0 flex-1">
                <p className="truncate font-semibold">
                  {profile.businessName || "Operator"}
                </p>
                <p className="truncate text-sm text-muted-foreground">
                  {profile.email}
                </p>
              </div>
              {planLoading ? (
                <Skeleton className="h-5 w-16 rounded-full" />
              ) : planName ? (
                <Badge className="bg-primary/10 text-primary">
                  {planName} plan
                </Badge>
              ) : null}
            </div>

            <dl className="divide-y border-t">
              <DetailRow label="Email" value={profile.email} />
              <DetailRow
                label="Business name"
                value={profile.businessName || "Not set"}
              />
              <DetailRow label="Account ID" value={profile.id} mono />
              <DetailRow label="Role" value={roleLabel(profile)} />
            </dl>

            <p className="text-xs text-muted-foreground">
              Profile fields are read-only for now; contact support to change
              your business name.
            </p>
          </>
        )}
      </CardContent>
    </Card>
  )
}
