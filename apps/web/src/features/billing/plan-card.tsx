"use client"

import { Check } from "lucide-react"

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
import { cn } from "@workspace/ui/lib/utils"

import {
  formatByteLimit,
  formatLimit,
  formatMYR,
  planActionLabel,
  planPriceCents,
  type BillingInterval,
  type PlanAction,
} from "./format"

type Plan = components["schemas"]["Plan"]

function limitsFor(plan: Plan): string[] {
  return [
    `${formatLimit(plan.limits.events)} events at a time`,
    `${formatLimit(plan.limits.photosPerEvent)} photos per event`,
    `${formatByteLimit(plan.limits.storageBytes)} storage`,
    `${formatLimit(plan.limits.retentionDays)} day retention`,
    plan.limits.apiAccess ? "API access" : "Dashboard only",
  ]
}

export function PlanCard({
  plan,
  interval,
  action,
  current,
  disabled,
  onAction,
}: {
  plan: Plan
  interval: BillingInterval
  action: PlanAction
  current: boolean
  disabled: boolean
  onAction: () => void
}) {
  const price = planPriceCents(plan, interval)
  const actionLabel = planActionLabel(action)
  const ariaLabel =
    action === "subscribe" || action === "upgrade" || action === "downgrade"
      ? `${actionLabel} to ${plan.name}`
      : `${actionLabel} ${plan.name}`

  return (
    <Card className={cn(current && "ring-2 ring-primary")}>
      <CardHeader>
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle className="text-sm font-semibold">{plan.name}</CardTitle>
          {current && (
            <Badge
              variant="outline"
              className="border-transparent bg-primary/10 text-primary"
            >
              Current
            </Badge>
          )}
        </div>
        <CardDescription className="flex items-baseline gap-1.5">
          <span className="text-xl font-semibold tracking-tight text-foreground">
            {formatMYR(price, plan.currency)}
          </span>
          <span className="text-xs font-medium">
            / {interval === "year" ? "year" : "month"}
          </span>
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col gap-4">
        <ul className="space-y-1.5 text-xs text-muted-foreground">
          {limitsFor(plan).map((limit) => (
            <li key={limit} className="flex items-start gap-2">
              <Check className="mt-0.5 size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
              <span>{limit}</span>
            </li>
          ))}
        </ul>
        <Button
          className="mt-auto w-full"
          variant={action === "current" ? "outline" : "default"}
          aria-label={ariaLabel}
          disabled={
            disabled || action === "current" || action === "unavailable"
          }
          onClick={onAction}
        >
          {actionLabel}
        </Button>
      </CardContent>
    </Card>
  )
}
