"use client"

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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@workspace/ui/components/table"

import { formatDate } from "@/features/events/format"

type AdminSubscription = components["schemas"]["AdminSubscription"]
type SubscriptionStatus = components["schemas"]["SubscriptionStatus"]

const STATUS_VARIANTS: Record<
  SubscriptionStatus,
  "default" | "secondary" | "destructive" | "outline"
> = {
  trialing: "outline",
  active: "secondary",
  past_due: "destructive",
  canceled: "outline",
  expired: "outline",
}

export function AdminSubscriptionsTable({
  subscriptions,
  hasNextPage,
  isFetchingNextPage,
  onLoadMore,
}: {
  subscriptions: AdminSubscription[]
  hasNextPage: boolean
  isFetchingNextPage: boolean
  onLoadMore: () => void
}) {
  return (
    <Card id="admin-subscriptions">
      <CardHeader>
        <CardTitle>Subscriptions</CardTitle>
        <CardDescription>Billing status across all tenants.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {subscriptions.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            No subscriptions yet.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead scope="col">Operator</TableHead>
                <TableHead scope="col">Plan</TableHead>
                <TableHead scope="col">Status</TableHead>
                <TableHead scope="col">Interval</TableHead>
                <TableHead scope="col">Current period end</TableHead>
                <TableHead scope="col">Cancels</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {subscriptions.map((subscription) => (
                <TableRow key={subscription.id}>
                  <TableCell className="font-medium">
                    {subscription.userEmail ?? "—"}
                  </TableCell>
                  <TableCell>{subscription.planId}</TableCell>
                  <TableCell>
                    <Badge variant={STATUS_VARIANTS[subscription.status]}>
                      {subscription.status}
                    </Badge>
                  </TableCell>
                  <TableCell>{subscription.interval}</TableCell>
                  <TableCell>
                    {subscription.currentPeriodEnd
                      ? formatDate(subscription.currentPeriodEnd)
                      : "—"}
                  </TableCell>
                  <TableCell>
                    {subscription.cancelAtPeriodEnd ? "Yes" : "No"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {hasNextPage && (
          <div className="flex justify-center">
            <Button
              variant="outline"
              disabled={isFetchingNextPage}
              onClick={onLoadMore}
            >
              {isFetchingNextPage ? "Loading..." : "Load more"}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
