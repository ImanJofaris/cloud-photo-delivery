import type { components } from "@workspace/api-client"

import { Badge } from "@workspace/ui/components/badge"
import { cn } from "@workspace/ui/lib/utils"

type PhotoStatus = components["schemas"]["PhotoStatus"]

const labels: Record<PhotoStatus, string> = {
  UPLOADING: "Uploading",
  PROCESSING: "Processing",
  READY: "Ready",
  FAILED: "Failed",
}

const styles: Record<PhotoStatus, string> = {
  UPLOADING: "bg-blue-500/10 text-blue-700 dark:text-blue-400",
  PROCESSING: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
  READY: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
  FAILED: "bg-destructive/10 text-destructive",
}

export function PhotoStatusBadge({
  status,
  className,
}: {
  status: PhotoStatus
  className?: string
}) {
  return (
    <Badge variant="outline" className={cn(styles[status], className)}>
      {labels[status]}
    </Badge>
  )
}
