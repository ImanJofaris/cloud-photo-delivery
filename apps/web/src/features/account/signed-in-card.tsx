"use client"

import { useRouter } from "next/navigation"

import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

import { useSession } from "@/lib/auth/session-provider"

export function SignedInCard() {
  const router = useRouter()
  const { logout } = useSession()

  async function handleSignOut() {
    try {
      await logout()
    } finally {
      router.replace("/login")
      router.refresh()
    }
  }

  return (
    <Card className="flex-1">
      <CardHeader>
        <CardTitle>Signed in</CardTitle>
        <CardDescription>
          One active session on this browser. The refresh token stays in a
          secure cookie.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void handleSignOut()}
        >
          Sign out
        </Button>
      </CardContent>
    </Card>
  )
}
