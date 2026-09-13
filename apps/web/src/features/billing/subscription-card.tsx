"use client"

import { CalendarClock, CreditCard } from "lucide-react"

import type { components } from "@workspace/api-client"

import { Badge } from "@workspace/ui/components/badge"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import { formatDate, formatDateTime } from "@/features/events/format"

import { isUsableSubscription } from "./format"
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

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2">
              <CreditCard className="size-4" />
              {view.plan.name} plan
              {subscription && <StatusBadge status={subscription.status} />}
            </CardTitle>
            <CardDescription>
              {subscription && usable
                ? `Billed ${subscription.interval === "year" ? "yearly" : "monthly"}`
                : "No paid subscription. The Free plan applies."}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            {subscription?.cancelAtPeriodEnd ? (
              <Button variant="outline" size="sm" onClick={onResume}>
                Resume
              </Button>
            ) : subscription && usable ? (
              <Button variant="outline" size="sm" onClick={onCancel}>
                Cancel subscription
              </Button>
            ) : null}
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-5">
        {subscription?.currentPeriodEnd && (
          <p className="flex items-center gap-2 text-sm text-muted-foreground">
            <CalendarClock className="size-4" />
            {subscription.cancelAtPeriodEnd ? (
              <>Cancels on {formatDate(subscription.currentPeriodEnd)}</>
            ) : (
              <>Renews on {formatDate(subscription.currentPeriodEnd)}</>
            )}
          </p>
        )}

        {subscription?.cancelAtPeriodEnd && (
          <p className="text-sm text-muted-foreground">
            Your plan stays active until{" "}
            {formatDateTime(subscription.currentPeriodEnd)}. Resume to keep it.
          </p>
        )}

        <UsageMeters plan={view.plan} usage={view.usage} />
      </CardContent>
    </Card>
  )
}
