"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { CircleAlert } from "lucide-react"
import { useRouter } from "next/navigation"
import { useForm, useWatch } from "react-hook-form"
import { toast } from "sonner"

import { Alert, AlertDescription } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import { Label } from "@workspace/ui/components/label"

import { publicApiBaseUrl } from "@/lib/env"

import { AuthCard } from "./auth-card"
import { PasswordInput } from "./password-input"
import { resetPasswordSchema, type ResetPasswordValues } from "./schemas"

export function ResetPasswordForm({ token }: { token: string }) {
  const router = useRouter()
  const form = useForm<ResetPasswordValues>({
    resolver: zodResolver(resetPasswordSchema),
    defaultValues: { password: "", confirmPassword: "" },
  })
  const password = useWatch({ control: form.control, name: "password" })
  const confirmPassword = useWatch({
    control: form.control,
    name: "confirmPassword",
  })
  const mismatch = !!confirmPassword && confirmPassword !== password

  async function onSubmit(values: ResetPasswordValues) {
    try {
      const response = await fetch(
        `${publicApiBaseUrl()}/api/v1/auth/password/reset-confirm`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token, password: values.password }),
        }
      )
      if (!response.ok) {
        form.setError("root", {
          message: "This reset link is invalid or has expired.",
        })
        return
      }
      toast.success("Password updated. Please sign in.")
      router.replace("/login")
    } catch {
      form.setError("root", {
        message: "Could not reach the server. Please try again.",
      })
    }
  }

  const { errors, isSubmitting } = form.formState

  return (
    <AuthCard
      title="Set a new password"
      description="Choose a password of at least 8 characters."
      footer="You will sign in with your new password."
    >
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className="space-y-2.5"
        noValidate
      >
        <div className="space-y-1.5">
          <Label htmlFor="password">New password</Label>
          <PasswordInput
            id="password"
            autoComplete="new-password"
            aria-invalid={Boolean(errors.password)}
            {...form.register("password")}
          />
          {errors.password && (
            <p className="text-sm text-destructive">
              {errors.password.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="confirmPassword">Confirm password</Label>
          <PasswordInput
            id="confirmPassword"
            autoComplete="new-password"
            aria-invalid={mismatch || Boolean(errors.confirmPassword)}
            aria-describedby={mismatch ? "password-mismatch" : undefined}
            {...form.register("confirmPassword")}
          />
          {!mismatch && errors.confirmPassword && (
            <p className="text-sm text-destructive">
              {errors.confirmPassword.message}
            </p>
          )}
        </div>

        {mismatch && (
          <Alert id="password-mismatch" variant="destructive">
            <CircleAlert aria-hidden="true" />
            <AlertDescription>Passwords do not match yet.</AlertDescription>
          </Alert>
        )}

        {errors.root && (
          <Alert variant="destructive">
            <CircleAlert aria-hidden="true" />
            <AlertDescription>{errors.root.message}</AlertDescription>
          </Alert>
        )}

        <Button type="submit" className="w-full" disabled={isSubmitting}>
          {isSubmitting ? "Saving..." : "Save new password"}
        </Button>
      </form>
    </AuthCard>
  )
}
