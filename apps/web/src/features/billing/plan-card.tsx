"use client"

import { Check } from "lucide-react"

import type { components } from "@workspace/api-client"

import { Badge } from "@workspace/ui/components/badge"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

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
    `${formatLimit(plan.limits.activeEvents)} active events`,
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
    <Card className={current ? "border-primary" : undefined}>
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle>{plan.name}</CardTitle>
          {current && <Badge>Current</Badge>}
        </div>
        <CardDescription>
          <span className="text-2xl font-semibold text-foreground">
            {formatMYR(price, plan.currency)}
          </span>{" "}
          / {interval === "year" ? "year" : "month"}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <ul className="space-y-1.5 text-sm">
          {limitsFor(plan).map((limit) => (
            <li key={limit} className="flex items-center gap-2">
              <Check className="size-4 shrink-0 text-muted-foreground" />
              <span>{limit}</span>
            </li>
          ))}
        </ul>
      </CardContent>
      <CardFooter>
        <Button
          className="w-full"
          variant={action === "current" ? "outline" : "default"}
          aria-label={ariaLabel}
          disabled={disabled || action === "current" || action === "unavailable"}
          onClick={onAction}
        >
          {actionLabel}
        </Button>
      </CardFooter>
    </Card>
  )
}
