"use client"

import * as React from "react"

import type { components } from "@workspace/api-client"

import { Button } from "@workspace/ui/components/button"
import { cn } from "@workspace/ui/lib/utils"

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
  const selectedInterval =
    hasUsable && view.subscription ? view.subscription.interval : interval

  return (
    <section
      id="plans"
      aria-labelledby="plans-heading"
      className="scroll-mt-6 space-y-4"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="plans-heading" className="text-lg font-semibold">
          Plans
        </h2>
        {!hasUsable && (
          <div
            role="group"
            aria-label="Billing interval"
            className="inline-flex items-center gap-0.5 rounded-xl bg-muted p-0.5"
          >
            <Button
              size="sm"
              variant="ghost"
              aria-pressed={interval === "month"}
              className={cn(
                "h-6 px-2.5 text-xs font-semibold",
                interval === "month" &&
                  "bg-card text-foreground shadow-xs hover:bg-card"
              )}
              onClick={() => setInterval("month")}
            >
              Monthly
            </Button>
            <Button
              size="sm"
              variant="ghost"
              aria-pressed={interval === "year"}
              className={cn(
                "h-6 px-2.5 text-xs font-semibold",
                interval === "year" &&
                  "bg-card text-foreground shadow-xs hover:bg-card"
              )}
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
              interval={selectedInterval}
              action={action}
              current={plan.id === view.plan.id}
              disabled={false}
              onAction={() => onAction(plan, action, interval)}
            />
          )
        })}
      </div>
    </section>
  )
}
