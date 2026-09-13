"use client"

import * as React from "react"

import type { components } from "@workspace/api-client"

import { Button } from "@workspace/ui/components/button"

import {
  isUsableSubscription,
  planAction,
  type BillingInterval,
  type PlanAction,
} from "./format"
import { PlanCard } from "./plan-card"

type Plan = components["schemas"]["Plan"]
type SubscriptionView = components["schemas"]["SubscriptionView"]

export function PlanGrid({
  plans,
  view,
  onAction,
}: {
  plans: Plan[]
  view: SubscriptionView
  onAction: (plan: Plan, action: PlanAction, interval: BillingInterval) => void
}) {
  const [interval, setInterval] = React.useState<BillingInterval>("month")
  const hasUsable = isUsableSubscription(view.subscription)

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">Plans</h2>
          <p className="text-sm text-muted-foreground">
            {hasUsable
              ? "Upgrades and downgrades keep your current billing interval."
              : "Pick a plan. Yearly billing is charged for twelve months."}
          </p>
        </div>
        {!hasUsable && (
          <div
            role="group"
            aria-label="Billing interval"
            className="inline-flex rounded-lg border p-1"
          >
            <Button
              size="sm"
              variant={interval === "month" ? "secondary" : "ghost"}
              onClick={() => setInterval("month")}
            >
              Monthly
            </Button>
            <Button
              size="sm"
              variant={interval === "year" ? "secondary" : "ghost"}
              onClick={() => setInterval("year")}
            >
              Yearly
            </Button>
          </div>
        )}
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {plans.map((plan) => {
          const action = planAction({
            plan,
            effectivePlanId: view.plan.id,
            effectivePriceCents: view.plan.priceCents,
            hasUsableSubscription: hasUsable,
          })
          return (
            <PlanCard
              key={plan.id}
              plan={plan}
              interval={
                hasUsable && view.subscription
                  ? view.subscription.interval
                  : interval
              }
              action={action}
              current={plan.id === view.plan.id}
              disabled={false}
              onAction={() => onAction(plan, action, interval)}
            />
          )
        })}
      </div>
    </div>
  )
}
