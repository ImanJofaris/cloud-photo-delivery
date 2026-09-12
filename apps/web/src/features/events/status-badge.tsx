import type { components } from "@workspace/api-client"

import { Badge } from "@workspace/ui/components/badge"
import { cn } from "@workspace/ui/lib/utils"

type EventStatus = components["schemas"]["EventStatus"]

const styles: Record<EventStatus, string> = {
  upcoming: "bg-blue-500/10 text-blue-700 dark:text-blue-400",
  active: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
  completed: "bg-muted text-muted-foreground",
  archived: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
}

export function EventStatusBadge({
  status,
  className,
}: {
  status: EventStatus
  className?: string
}) {
  return (
    <Badge
      variant="outline"
      className={cn("capitalize", styles[status], className)}
    >
      {status}
    </Badge>
  )
}
