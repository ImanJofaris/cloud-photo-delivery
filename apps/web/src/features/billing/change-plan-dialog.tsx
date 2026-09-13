"use client"

import * as React from "react"
import { toast } from "sonner"

import type { components } from "@workspace/api-client"

import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@workspace/ui/components/dialog"

import {
  useCancelSubscription,
  useDowngrade,
  useResumeSubscription,
  useSubscribe,
  useUpgrade,
} from "./api"
import { billingErrorMessage } from "./errors"
import {
  formatMYR,
  isHttpUrl,
  planPriceCents,
  type BillingInterval,
} from "./format"

type Plan = components["schemas"]["Plan"]

export type ChangePlanMode =
  | "subscribe"
  | "upgrade"
  | "downgrade"
  | "cancel"
  | "resume"

export function openCheckout(url: string): "opened" | "offline" {
  if (isHttpUrl(url)) {
    window.open(url, "_blank", "noopener,noreferrer")
    return "opened"
  }
  return "offline"
}

const TITLES: Record<ChangePlanMode, string> = {
  subscribe: "Subscribe",
  upgrade: "Upgrade plan",
  downgrade: "Downgrade plan",
  cancel: "Cancel subscription",
  resume: "Resume subscription",
}

const ACTIONS: Record<ChangePlanMode, string> = {
  subscribe: "Subscribe",
  upgrade: "Upgrade",
  downgrade: "Downgrade",
  cancel: "Cancel subscription",
  resume: "Resume subscription",
}

export function ChangePlanDialog({
  mode,
  plan,
  interval,
  open,
  onOpenChange,
}: {
  mode: ChangePlanMode
  plan: Plan | null
  interval: BillingInterval
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const subscribe = useSubscribe()
  const upgrade = useUpgrade()
  const downgrade = useDowngrade()
  const cancel = useCancelSubscription()
  const resume = useResumeSubscription()
  const [offline, setOffline] = React.useState(false)

  function handleOpenChange(next: boolean) {
    if (!next) setOffline(false)
    onOpenChange(next)
  }

  const pending =
    subscribe.isPending ||
    upgrade.isPending ||
    downgrade.isPending ||
    cancel.isPending ||
    resume.isPending

  let title = TITLES[mode]
  let description = ""
  if (mode === "subscribe" && plan) {
    title = `Subscribe to ${plan.name}?`
    description = `You will be billed ${formatMYR(planPriceCents(plan, interval), plan.currency)} per ${interval === "year" ? "year" : "month"}.`
  } else if (mode === "upgrade" && plan) {
    title = `Upgrade to ${plan.name}?`
    description = "Your plan changes immediately."
  } else if (mode === "downgrade" && plan) {
    title = `Downgrade to ${plan.name}?`
    description =
      "Your data is kept. New events and uploads are blocked while you are over the target plan's limits."
  } else if (mode === "cancel") {
    description =
      "Your plan stays active until the end of the current period, then the account moves to Free."
  } else if (mode === "resume") {
    description =
      "The subscription renews at the end of the current period instead of ending."
  }

  async function run() {
    try {
      if (mode === "subscribe" && plan) {
        const result = await subscribe.mutateAsync({
          planId: plan.id,
          interval,
        })
        if (openCheckout(result.checkout.url) === "offline") {
          setOffline(true)
          return
        }
        toast.success("Subscription active")
      } else if (mode === "upgrade" && plan) {
        await upgrade.mutateAsync({ planId: plan.id })
        toast.success(`Upgraded to ${plan.name}`)
      } else if (mode === "downgrade" && plan) {
        await downgrade.mutateAsync({ planId: plan.id })
        toast.success(`Downgraded to ${plan.name}`)
      } else if (mode === "cancel") {
        await cancel.mutateAsync()
        toast.success("Subscription will cancel at period end")
      } else if (mode === "resume") {
        await resume.mutateAsync()
        toast.success("Subscription resumed")
      }
      handleOpenChange(false)
    } catch (error) {
      toast.error(billingErrorMessage(error))
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && (
            <DialogDescription>{description}</DialogDescription>
          )}
        </DialogHeader>

        {offline ? (
          <div className="space-y-3">
            <Alert>
              <AlertTitle>Offline billing</AlertTitle>
              <AlertDescription>
                The subscription is active now. Pay the open invoice shown in
                the Invoices section; no online payment is required.
              </AlertDescription>
            </Alert>
            <DialogFooter>
              <Button type="button" onClick={() => handleOpenChange(false)}>
                Done
              </Button>
            </DialogFooter>
          </div>
        ) : (
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={() => handleOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant={mode === "cancel" ? "destructive" : "default"}
              disabled={pending}
              onClick={() => void run()}
            >
              {pending ? "Working..." : ACTIONS[mode]}
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
