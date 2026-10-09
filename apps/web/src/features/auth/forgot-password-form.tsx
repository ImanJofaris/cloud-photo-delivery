"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { CircleCheck } from "lucide-react"
import * as React from "react"
import { useForm } from "react-hook-form"

import { Alert, AlertDescription } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"

import { publicApiBaseUrl } from "@/lib/env"

import { AuthCard, AuthFooterLink } from "./auth-card"
import { forgotPasswordSchema, type ForgotPasswordValues } from "./schemas"

export function ForgotPasswordForm() {
  const form = useForm<ForgotPasswordValues>({
    resolver: zodResolver(forgotPasswordSchema),
    defaultValues: { email: "" },
  })
  const [sent, setSent] = React.useState(false)

  async function onSubmit(values: ForgotPasswordValues) {
    try {
      await fetch(`${publicApiBaseUrl()}/api/v1/auth/password/reset-request`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(values),
      })
      setSent(true)
    } catch {
      form.setError("root", {
        message: "Could not reach the server. Please try again.",
      })
    }
  }

  const { errors, isSubmitting } = form.formState
  const submitLabel = sent ? "Resend link" : "Send reset link"

  return (
    <AuthCard
      title="Reset password"
      description="Enter your email and we will send a reset link."
      footer={<AuthFooterLink href="/login">Back to sign in</AuthFooterLink>}
    >
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className="space-y-2.5"
        noValidate
      >
        <div className="space-y-1.5">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            aria-invalid={Boolean(errors.email)}
            {...form.register("email")}
          />
          {errors.email && (
            <p className="text-sm text-destructive">{errors.email.message}</p>
          )}
        </div>

        {sent && (
          <Alert
            role="status"
            className="border-transparent bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"
          >
            <CircleCheck aria-hidden="true" />
            <AlertDescription className="text-current">
              If that email is registered, a reset link is on its way.
            </AlertDescription>
          </Alert>
        )}

        {errors.root && (
          <Alert variant="destructive">
            <AlertDescription>{errors.root.message}</AlertDescription>
          </Alert>
        )}

        <Button
          type="submit"
          variant={sent ? "outline" : "default"}
          className="w-full"
          disabled={isSubmitting}
        >
          {isSubmitting ? "Sending..." : submitLabel}
        </Button>
      </form>
    </AuthCard>
  )
}
