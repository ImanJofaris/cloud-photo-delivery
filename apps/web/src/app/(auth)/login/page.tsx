import { LoginForm } from "@/features/auth/login-form"

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ next?: string }>
}) {
  const { next } = await searchParams
  const target =
    next && next.startsWith("/") && !next.startsWith("//") ? next : "/dashboard"

  return <LoginForm next={target} />
}
