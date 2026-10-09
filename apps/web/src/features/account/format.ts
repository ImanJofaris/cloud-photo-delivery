import type { components } from "@workspace/api-client"

import {
  formatMYR,
  isUsableSubscription,
  planPriceCents,
} from "@/features/billing/format"
import { formatDate } from "@/features/events/format"

type Profile = components["schemas"]["Profile"]
type SubscriptionView = components["schemas"]["SubscriptionView"]

export function initials(value: string | null | undefined): string {
  if (!value) return ""
  return value
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("")
}

export function roleLabel(profile: Pick<Profile, "isAdmin">): string {
  return profile.isAdmin ? "Admin" : "Operator"
}

export function planSummary(view: SubscriptionView): string {
  const { plan, subscription } = view

  if (!subscription || !isUsableSubscription(subscription)) {
    return `${plan.name} plan · no paid subscription`
  }

  const price = formatMYR(
    planPriceCents(plan, subscription.interval),
    plan.currency
  )
  const interval = subscription.interval === "year" ? "year" : "month"
  const period = subscription.currentPeriodEnd
    ? ` · ${subscription.cancelAtPeriodEnd ? "cancels" : "renews"} ${formatDate(
        subscription.currentPeriodEnd
      )}`
    : ""

  return `${plan.name} · ${price} / ${interval}${period}`
}
