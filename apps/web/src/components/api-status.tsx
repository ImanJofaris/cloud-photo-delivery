import { cn } from "@workspace/ui/lib/utils"

const statusStyles = {
  online: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
  offline: "bg-destructive/10 text-destructive",
  unconfigured: "bg-muted text-muted-foreground",
} as const

const statusLabels = {
  online: "API online",
  offline: "API offline",
  unconfigured: "API not configured",
} as const

export type ApiStatusValue = keyof typeof statusStyles

export function ApiStatus({ status }: { status: ApiStatusValue }) {
  return (
    <span
      data-testid="api-status"
      className={cn(
        "rounded-full px-3 py-1 text-xs font-medium",
        statusStyles[status]
      )}
    >
      {statusLabels[status]}
    </span>
  )
}
