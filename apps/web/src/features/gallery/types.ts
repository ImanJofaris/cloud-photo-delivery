import type { components } from "@workspace/api-client"

type PublicEvent = components["schemas"]["PublicEvent"]
type PublicPhoto = components["schemas"]["PublicPhoto"]

export type GalleryEvent = Omit<
  PublicEvent,
  "name" | "allowDownload" | "allowOriginalDownload"
> & {
  name: string
  allowDownload: boolean
  allowOriginalDownload: boolean
}

export type GalleryPhoto = Omit<PublicPhoto, "id" | "variants"> & {
  id: string
  variants: string[]
}

export function toGalleryEvent(event: PublicEvent): GalleryEvent {
  return {
    ...event,
    name: event.name ?? "Event gallery",
    allowDownload: event.allowDownload ?? false,
    allowOriginalDownload: event.allowOriginalDownload ?? false,
  }
}

export function toGalleryPhoto(photo: PublicPhoto): GalleryPhoto | null {
  if (!photo.id) return null
  return { ...photo, id: photo.id, variants: photo.variants ?? [] }
}
