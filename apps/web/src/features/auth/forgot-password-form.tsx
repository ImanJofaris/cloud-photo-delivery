"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import * as React from "react"
import { useForm } from "react-hook-form"

import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"

import { publicApiBaseUrl } from "@/lib/env"

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

  return (
    <Card>
      <CardHeader>
        <CardTitle>Reset password</CardTitle>
        <CardDescription>
          We will email you a link to set a new password.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {sent ? (
          <p className="text-sm text-muted-foreground">
            If that email is registered, a reset link is on its way.
          </p>
        ) : (
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4"
            noValidate
          >
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                autoComplete="email"
                aria-invalid={Boolean(errors.email)}
                {...form.register("email")}
              />
              {errors.email && (
                <p className="text-sm text-destructive">
                  {errors.email.message}
                </p>
              )}
            </div>

            {errors.root && (
              <p className="text-sm text-destructive">{errors.root.message}</p>
            )}

            <Button type="submit" className="w-full" disabled={isSubmitting}>
              {isSubmitting ? "Sending..." : "Send reset link"}
            </Button>
          </form>
        )}

        <p className="mt-4 text-center text-sm text-muted-foreground">
          <a
            href="/login"
            className="text-foreground underline-offset-4 hover:underline"
          >
            Back to sign in
          </a>
        </p>
      </CardContent>
    </Card>
  )
}
