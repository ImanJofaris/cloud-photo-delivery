"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"

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

import { authErrorMessage } from "@/lib/auth/errors"
import { useSession } from "@/lib/auth/session-provider"

import { signupSchema, type SignupValues } from "./schemas"

export function SignupForm() {
  const { signup } = useSession()
  const router = useRouter()
  const form = useForm<SignupValues>({
    resolver: zodResolver(signupSchema),
    defaultValues: { businessName: "", email: "", password: "" },
  })

  async function onSubmit(values: SignupValues) {
    try {
      await signup({
        email: values.email,
        password: values.password,
        businessName: values.businessName || undefined,
      })
      router.replace("/dashboard")
      router.refresh()
    } catch (error) {
      form.setError("root", { message: authErrorMessage(error) })
    }
  }

  const { errors, isSubmitting } = form.formState

  return (
    <Card>
      <CardHeader>
        <CardTitle>Create account</CardTitle>
        <CardDescription>
          Start delivering event galleries to your clients.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className="space-y-4"
          noValidate
        >
          <div className="space-y-2">
            <Label htmlFor="businessName">Business name (optional)</Label>
            <Input
              id="businessName"
              autoComplete="organization"
              {...form.register("businessName")}
            />
            {errors.businessName && (
              <p className="text-sm text-destructive">
                {errors.businessName.message}
              </p>
            )}
          </div>

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
              <p className="text-sm text-destructive">{errors.email.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
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

          {errors.root && (
            <Alert variant="destructive">
              <AlertDescription>{errors.root.message}</AlertDescription>
            </Alert>
          )}

          <Button type="submit" className="w-full" disabled={isSubmitting}>
            {isSubmitting ? "Creating account..." : "Create account"}
          </Button>
        </form>

        <p className="mt-4 text-center text-sm text-muted-foreground">
          Already registered?{" "}
          <a
            href="/login"
            className="text-foreground underline-offset-4 hover:underline"
          >
            Sign in
          </a>
        </p>
      </CardContent>
    </Card>
  )
}
