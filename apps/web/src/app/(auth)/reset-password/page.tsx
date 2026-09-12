import { Alert, AlertDescription } from "@workspace/ui/components/alert"

import { ResetPasswordForm } from "@/features/auth/reset-password-form"

export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string }>
}) {
  const { token } = await searchParams

  if (!token) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          This reset link is missing its token. Request a new link from the
          sign-in page.
        </AlertDescription>
      </Alert>
    )
  }

  return <ResetPasswordForm token={token} />
}
