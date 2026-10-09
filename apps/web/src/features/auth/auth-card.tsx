import * as React from "react"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"

type AuthCardProps = {
  title: string
  description: React.ReactNode
  children: React.ReactNode
  footer?: React.ReactNode
}

export function AuthCard({
  title,
  description,
  children,
  footer,
}: AuthCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="font-bold">{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>
        {children}
        {footer ? (
          <p className="mt-3 text-center text-sm text-muted-foreground">
            {footer}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}

export function AuthFooterLink({
  href,
  children,
}: {
  href: string
  children: React.ReactNode
}) {
  return (
    <a
      href={href}
      className="font-semibold text-foreground underline-offset-4 hover:underline"
    >
      {children}
    </a>
  )
}
