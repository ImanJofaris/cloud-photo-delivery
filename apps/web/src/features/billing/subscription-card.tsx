"use client"

import type { components } from "@workspace/api-client"

import { Badge } from "@workspace/ui/components/badge"
import { Button } from "@workspace/ui/components/button"
import { Card, CardContent } from "@workspace/ui/components/card"

import { formatDate } from "@/features/events/format"

import { formatMYR, isUsableSubscription, planPriceCents } from "./format"
import { UsageMeters } from "./usage-meters"

type SubscriptionView = components["schemas"]["SubscriptionView"]
type SubscriptionStatus = components["schemas"]["SubscriptionStatus"]

const STATUS_LABELS: Record<SubscriptionStatus, string> = {
  trialing: "Trial",
  active: "Active",
  past_due: "Past due",
  canceled: "Canceled",
  expired: "Expired",
}

function StatusBadge({ status }: { status: SubscriptionStatus }) {
  const variant =
    status === "active"
      ? "secondary"
      : status === "past_due"
        ? "destructive"
        : "outline"
  return <Badge variant={variant}>{STATUS_LABELS[status]}</Badge>
}

export function SubscriptionCard({
  view,
  onCancel,
  onResume,
}: {
  view: SubscriptionView
  onCancel: () => void
  onResume: () => void
}) {
  const subscription = view.subscription
  const usable = isUsableSubscription(subscription)
  const interval =
    usable && subscription ? subscription.interval : view.plan.interval
  const price = planPriceCents(view.plan, interval)

  return (
    <Card>
      <CardContent className="flex flex-col gap-6 lg:flex-row lg:items-center lg:gap-10">
        <div className="space-y-3 lg:w-72 lg:shrink-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-lg font-semibold">{view.plan.name}</h2>
            <Badge
              variant="outline"
              className="border-transparent bg-primary/10 text-primary"
            >
              Current plan
            </Badge>
            {subscription && subscription.status !== "active" && (
              <StatusBadge status={subscription.status} />
            )}
          </div>

          <p className="text-sm text-muted-foreground">
            {subscription && usable ? (
              <>
                {formatMYR(price, view.plan.currency)} /{" "}
                {interval === "year" ? "year" : "month"}
                {subscription.currentPeriodEnd && (
                  <>
                    {" · "}
                    {subscription.cancelAtPeriodEnd ? "ends" : "renews"}{" "}
                    {formatDate(subscription.currentPeriodEnd)}
                  </>
                )}
              </>
            ) : (
              "No paid subscription. The Free plan applies."
            )}
          </p>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              nativeButton={false}
              render={<a href="#plans" />}
            >
              Change plan
            </Button>
            {subscription?.cancelAtPeriodEnd ? (
              <Button size="sm" variant="ghost" onClick={onResume}>
                Resume
              </Button>
            ) : subscription && usable ? (
              <Button
                size="sm"
                variant="ghost"
                aria-label="Cancel subscription"
                onClick={onCancel}
              >
                Cancel
              </Button>
            ) : null}
          </div>
        </div>

        <div className="flex-1">
          <UsageMeters plan={view.plan} usage={view.usage} />
        </div>
      </CardContent>
    </Card>
  )
}
