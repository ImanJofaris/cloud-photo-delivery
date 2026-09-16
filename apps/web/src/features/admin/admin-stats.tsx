import type { components } from "@workspace/api-client"

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import { formatCount } from "@/features/analytics/format"
import { formatMYR } from "@/features/billing/format"

import { formatBytes } from "./format"

type AdminStats = components["schemas"]["AdminStats"]

export const ADMIN_STATS_CARDS: {
  key: keyof AdminStats
  label: string
  format: (value: number) => string
}[] = [
  { key: "users", label: "Operators", format: formatCount },
  { key: "events", label: "Events", format: formatCount },
  { key: "photos", label: "Photos", format: formatCount },
  { key: "storageBytes", label: "Storage", format: formatBytes },
  { key: "revenueCents", label: "Revenue", format: formatMYR },
  { key: "subscriptions", label: "Subscriptions", format: formatCount },
]

export function AdminStatsGrid({
  stats,
  loading = false,
}: {
  stats?: AdminStats
  loading?: boolean
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {ADMIN_STATS_CARDS.map((card) => (
        <Card key={card.key}>
          <CardHeader className="pb-2">
            <CardDescription>{card.label}</CardDescription>
            <CardTitle className="text-2xl">
              {loading || !stats ? "—" : card.format(stats[card.key])}
            </CardTitle>
          </CardHeader>
        </Card>
      ))}
    </div>
  )
}
