"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"

import { Alert, AlertDescription } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"

import { authErrorMessage } from "@/lib/auth/errors"
import { useSession } from "@/lib/auth/session-provider"

import { AuthCard, AuthFooterLink } from "./auth-card"
import { PasswordInput } from "./password-input"
import { loginSchema, type LoginValues } from "./schemas"

export function LoginForm({ next = "/dashboard" }: { next?: string }) {
  const { login } = useSession()
  const router = useRouter()
  const form = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  })

  async function onSubmit(values: LoginValues) {
    try {
      await login(values)
      router.replace(next)
      router.refresh()
    } catch (error) {
      form.setError("root", { message: authErrorMessage(error) })
    }
  }

  const { errors, isSubmitting } = form.formState

  return (
    <AuthCard
      title="Sign in"
      description="Use your operator account to manage events."
      footer={
        <>
          No account? <AuthFooterLink href="/signup">Create one</AuthFooterLink>
        </>
      }
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

        <div className="space-y-1.5">
          <Label htmlFor="password">Password</Label>
          <PasswordInput
            id="password"
            autoComplete="current-password"
            aria-invalid={Boolean(errors.password)}
            {...form.register("password")}
          />
          {errors.password && (
            <p className="text-sm text-destructive">
              {errors.password.message}
            </p>
          )}
          <div className="text-right">
            <a
              href="/forgot-password"
              className="text-xs text-muted-foreground underline underline-offset-4 hover:text-foreground"
            >
              Forgot password?
            </a>
          </div>
        </div>

        {errors.root && (
          <Alert variant="destructive">
            <AlertDescription>{errors.root.message}</AlertDescription>
          </Alert>
        )}

        <Button type="submit" className="w-full" disabled={isSubmitting}>
          {isSubmitting ? "Signing in..." : "Sign in"}
        </Button>
      </form>
    </AuthCard>
  )
}
