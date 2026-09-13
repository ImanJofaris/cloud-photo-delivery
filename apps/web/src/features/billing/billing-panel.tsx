"use client"

import * as React from "react"

import type { components } from "@workspace/api-client"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Skeleton } from "@workspace/ui/components/skeleton"

import { useInvoices, usePlans, useSubscription } from "./api"
import { ChangePlanDialog, type ChangePlanMode } from "./change-plan-dialog"
import type { BillingInterval, PlanAction } from "./format"
import { InvoicesTable } from "./invoices-table"
import { PlanGrid } from "./plan-grid"
import { SubscriptionCard } from "./subscription-card"

type Plan = components["schemas"]["Plan"]

type DialogState = {
  mode: ChangePlanMode
  plan: Plan | null
  interval: BillingInterval
}

export function BillingPanel() {
  const plansQuery = usePlans()
  const subscriptionQuery = useSubscription()
  const invoicesQuery = useInvoices()
  const [dialog, setDialog] = React.useState<DialogState | null>(null)

  const invoices =
    invoicesQuery.data?.pages.flatMap((page) => page.items) ?? []

  function handleAction(
    plan: Plan,
    action: PlanAction,
    interval: BillingInterval
  ) {
    if (action === "current" || action === "unavailable") return
    setDialog({ mode: action, plan, interval })
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">Billing</h1>
          <p className="text-sm text-muted-foreground">
            Your plan, usage, and invoices.
          </p>
        </div>
      </div>

      {subscriptionQuery.isPending && (
        <div className="space-y-3">
          <Skeleton className="h-48 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      )}

      {subscriptionQuery.isError && (
        <Card>
          <CardHeader>
            <CardTitle>Could not load billing</CardTitle>
            <CardDescription>
              Refresh the page to try again.
            </CardDescription>
          </CardHeader>
          <CardContent />
        </Card>
      )}

      {subscriptionQuery.data && (
        <>
          <SubscriptionCard
            view={subscriptionQuery.data}
            onCancel={() =>
              setDialog({
                mode: "cancel",
                plan: null,
                interval: "month",
              })
            }
            onResume={() =>
              setDialog({
                mode: "resume",
                plan: null,
                interval: "month",
              })
            }
          />

          {plansQuery.data && (
            <PlanGrid
              plans={plansQuery.data.items}
              view={subscriptionQuery.data}
              onAction={handleAction}
            />
          )}

          <InvoicesTable
            invoices={invoices}
            hasNextPage={Boolean(invoicesQuery.hasNextPage)}
            isFetchingNextPage={invoicesQuery.isFetchingNextPage}
            onLoadMore={() => void invoicesQuery.fetchNextPage()}
          />
        </>
      )}

      <ChangePlanDialog
        mode={dialog?.mode ?? "subscribe"}
        plan={dialog?.plan ?? null}
        interval={dialog?.interval ?? "month"}
        open={dialog !== null}
        onOpenChange={(open) => {
          if (!open) setDialog(null)
        }}
      />
    </div>
  )
}
