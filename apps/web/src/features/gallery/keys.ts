export const galleryKeys = {
  all: (slug: string) => ["gallery", slug] as const,
  event: (slug: string) => [...galleryKeys.all(slug), "event"] as const,
  photos: (slug: string) => [...galleryKeys.all(slug), "photos"] as const,
  url: (slug: string, photoId: string, variant: string) =>
    [...galleryKeys.all(slug), "url", photoId, variant] as const,
}
