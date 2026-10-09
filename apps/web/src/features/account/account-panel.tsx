"use client"

import { useSubscription } from "@/features/billing/api"

import { useProfile } from "./api"
import { PlanSummaryCard } from "./plan-summary-card"
import { ProfileCard } from "./profile-card"
import { SignedInCard } from "./signed-in-card"

export function AccountPanel() {
  const profileQuery = useProfile()
  const subscriptionQuery = useSubscription()
  const view = subscriptionQuery.data

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Account</h1>
        <p className="text-sm text-muted-foreground">Your operator profile.</p>
      </div>

      <div className="grid gap-4 lg:grid-cols-[1.4fr_1fr]">
        <ProfileCard
          profile={profileQuery.data}
          loading={profileQuery.isPending}
          error={profileQuery.isError}
          planName={view?.plan.name}
          planLoading={subscriptionQuery.isPending}
        />

        <div className="flex flex-col gap-4">
          <PlanSummaryCard
            view={view}
            loading={subscriptionQuery.isPending}
            error={subscriptionQuery.isError}
          />
          <SignedInCard />
        </div>
      </div>
    </div>
  )
}
