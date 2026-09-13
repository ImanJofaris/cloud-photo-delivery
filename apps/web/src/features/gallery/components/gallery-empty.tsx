import { ImageIcon } from "lucide-react"

export function GalleryEmpty({ eventName }: { eventName: string }) {
  return (
    <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed px-6 py-16 text-center">
      <ImageIcon
        className="size-8 text-muted-foreground"
        aria-hidden="true"
      />
      <p className="text-sm font-medium">No photos yet</p>
      <p className="max-w-xs text-sm text-muted-foreground">
        Photos from {eventName} will appear here as they are uploaded.
      </p>
    </div>
  )
}
