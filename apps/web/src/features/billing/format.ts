import type { components } from "@workspace/api-client"

import { formatBytes } from "@/features/events/format"

type Plan = components["schemas"]["Plan"]
type Subscription = components["schemas"]["Subscription"]

export type PlanAction =
  | "current"
  | "subscribe"
  | "upgrade"
  | "downgrade"
  | "unavailable"

export type BillingInterval = "month" | "year"

export function formatMYR(cents: number, currency = "MYR"): string {
  return new Intl.NumberFormat("en-MY", {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
  }).format(cents / 100)
}

export function formatLimit(value: number): string {
  return value === 0 ? "Unlimited" : value.toLocaleString("en-MY")
}

export function formatByteLimit(bytes: number): string {
  return bytes === 0 ? "Unlimited" : formatBytes(bytes)
}

export function isUsableStatus(status: Subscription["status"]): boolean {
  return status === "trialing" || status === "active" || status === "past_due"
}

export function isUsableSubscription(
  subscription: Subscription | null | undefined
): boolean {
  return Boolean(subscription && isUsableStatus(subscription.status))
}

export function planPriceCents(plan: Plan, interval: BillingInterval): number {
  return interval === "year" ? plan.priceCents * 12 : plan.priceCents
}

export function planAction({
  plan,
  effectivePlanId,
  effectivePriceCents,
  hasUsableSubscription,
}: {
  plan: Plan
  effectivePlanId: string
  effectivePriceCents: number
  hasUsableSubscription: boolean
}): PlanAction {
  if (!plan.active) return "unavailable"
  if (!hasUsableSubscription) {
    return plan.id === effectivePlanId ? "current" : "subscribe"
  }
  if (plan.id === effectivePlanId) return "current"
  if (plan.priceCents > effectivePriceCents) return "upgrade"
  if (plan.priceCents < effectivePriceCents) return "downgrade"
  return "current"
}

export function planActionLabel(action: PlanAction): string {
  switch (action) {
    case "current":
      return "Current plan"
    case "subscribe":
      return "Subscribe"
    case "upgrade":
      return "Upgrade"
    case "downgrade":
      return "Downgrade"
    case "unavailable":
      return "Unavailable"
  }
}

export function isHttpUrl(url: string): boolean {
  return /^https?:\/\//i.test(url)
}

export function usagePercent(used: number, limit: number): number {
  if (limit <= 0) return 0
  return Math.min(100, Math.round((used / limit) * 100))
}
