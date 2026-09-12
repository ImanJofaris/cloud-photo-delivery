"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
import { toast } from "sonner"

import { Alert, AlertDescription } from "@workspace/ui/components/alert"
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

import { resetPasswordSchema, type ResetPasswordValues } from "./schemas"

export function ResetPasswordForm({ token }: { token: string }) {
  const router = useRouter()
  const form = useForm<ResetPasswordValues>({
    resolver: zodResolver(resetPasswordSchema),
    defaultValues: { password: "", confirmPassword: "" },
  })

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
    <Card>
      <CardHeader>
        <CardTitle>Set a new password</CardTitle>
        <CardDescription>
          Choose a password of at least 8 characters.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className="space-y-4"
          noValidate
        >
          <div className="space-y-2">
            <Label htmlFor="password">New password</Label>
            <Input
              id="password"
              type="password"
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

          <div className="space-y-2">
            <Label htmlFor="confirmPassword">Confirm password</Label>
            <Input
              id="confirmPassword"
              type="password"
              autoComplete="new-password"
              aria-invalid={Boolean(errors.confirmPassword)}
              {...form.register("confirmPassword")}
            />
            {errors.confirmPassword && (
              <p className="text-sm text-destructive">
                {errors.confirmPassword.message}
              </p>
            )}
          </div>

          {errors.root && (
            <Alert variant="destructive">
              <AlertDescription>{errors.root.message}</AlertDescription>
            </Alert>
          )}

          <Button type="submit" className="w-full" disabled={isSubmitting}>
            {isSubmitting ? "Updating..." : "Update password"}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
