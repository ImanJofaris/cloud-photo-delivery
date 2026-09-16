"use client"

import * as React from "react"
import Link from "next/link"
import { toast } from "sonner"

import type { components } from "@workspace/api-client"
import { Alert, AlertDescription } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import { isPlanLimitReached } from "@/lib/api-errors"

import { useTransitionEvent } from "./api"
import { eventErrorMessage } from "./errors"

type EventStatus = components["schemas"]["EventStatus"]

const ACTIONS: Partial<
  Record<
    EventStatus,
    { to: EventStatus; label: string; pending: string; success: string }
  >
> = {
  upcoming: {
    to: "active",
    label: "Mark active",
    pending: "Starting...",
    success: "Event is now active",
  },
  active: {
    to: "completed",
    label: "Mark completed",
    pending: "Completing...",
    success: "Event marked completed",
  },
}

export function EventStatusCard({
  eventId,
  status,
}: {
  eventId: string
  status: EventStatus
}) {
  const transition = useTransitionEvent(eventId)
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null)
  const [planLimitReached, setPlanLimitReached] = React.useState(false)

  const action = ACTIONS[status]
  if (!action) return null
  const { to, label, pending, success } = action

  async function handleTransition() {
    setErrorMessage(null)
    setPlanLimitReached(false)
    try {
      await transition.mutateAsync(to)
      toast.success(success)
    } catch (transitionError) {
      setErrorMessage(eventErrorMessage(transitionError))
      setPlanLimitReached(isPlanLimitReached(transitionError))
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Event status</CardTitle>
        <CardDescription>
          Events you can still share count toward your plan&apos;s limit.
          Archive one to free a slot.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Button
          onClick={() => void handleTransition()}
          disabled={transition.isPending}
        >
          {transition.isPending ? pending : label}
        </Button>

        {errorMessage !== null && (
          <Alert variant="destructive">
            <AlertDescription>
              <p>{errorMessage}</p>
              {planLimitReached && (
                <Button
                  render={<Link href="/billing" />}
                  nativeButton={false}
                  size="sm"
                  variant="outline"
                  className="mt-2"
                >
                  View plans
                </Button>
              )}
            </AlertDescription>
          </Alert>
        )}
      </CardContent>
    </Card>
  )
}
