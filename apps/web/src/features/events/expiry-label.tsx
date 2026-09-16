import { cn } from "@workspace/ui/lib/utils"

import { expiryFromNow, type ExpiryTone } from "./format"

const toneClasses: Record<ExpiryTone, string> = {
  neutral: "text-muted-foreground",
  warning: "text-amber-600 dark:text-amber-400",
  expired: "text-destructive",
}

export function expiryToneClass(tone: ExpiryTone): string {
  return toneClasses[tone]
}

export function ExpiryLabel({
  expiresAt,
  now,
  className,
}: {
  expiresAt: string | null | undefined
  now?: number
  className?: string
}) {
  const expiry = expiryFromNow(expiresAt, now)

  return (
    <span className={cn(toneClasses[expiry.tone], className)}>
      {expiry.label}
    </span>
  )
}
