"use client"

import { Lock } from "lucide-react"
import * as React from "react"

import { Button } from "@workspace/ui/components/button"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"

import { galleryErrorMessage, isNotFound } from "../errors"

export function UnlockGate({
  eventName,
  isPending,
  error,
  onSubmit,
}: {
  eventName: string
  isPending: boolean
  error: unknown
  onSubmit: (password: string) => void
}) {
  const [password, setPassword] = React.useState("")

  if (isNotFound(error)) {
    return (
      <div className="mx-auto max-w-sm rounded-xl border px-6 py-10 text-center">
        <p className="text-sm font-medium">
          {galleryErrorMessage(error)}
        </p>
      </div>
    )
  }

  return (
    <form
      className="mx-auto flex max-w-sm flex-col gap-4 rounded-xl border px-6 py-10"
      onSubmit={(event) => {
        event.preventDefault()
        if (password) onSubmit(password)
      }}
    >
      <div className="flex flex-col items-center gap-2 text-center">
        <span className="flex size-10 items-center justify-center rounded-full bg-muted">
          <Lock className="size-5" aria-hidden="true" />
        </span>
        <h2 className="text-lg font-semibold">This gallery is protected</h2>
        <p className="text-sm text-muted-foreground">
          Enter the password to view {eventName}.
        </p>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="gallery-password">Password</Label>
        <Input
          id="gallery-password"
          type="password"
          value={password}
          autoComplete="current-password"
          aria-invalid={Boolean(error)}
          onChange={(event) => setPassword(event.target.value)}
        />
        {error ? (
          <p role="alert" className="text-sm text-destructive">
            {galleryErrorMessage(error)}
          </p>
        ) : null}
      </div>

      <Button type="submit" disabled={isPending || password.length === 0}>
        {isPending ? "Unlocking..." : "Unlock gallery"}
      </Button>
    </form>
  )
}
