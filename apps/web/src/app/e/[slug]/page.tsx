import { notFound } from "next/navigation"

import {
  ApiError,
  createApiClient,
  unwrapEnvelope,
  type components,
} from "@workspace/api-client"

import { GalleryShell } from "@/features/gallery/components/gallery-shell"
import { toGalleryEvent } from "@/features/gallery/types"
import { API_VERSION_PATH, serverApiBaseUrl } from "@/lib/env"

type PublicEvent = components["schemas"]["PublicEvent"]

async function fetchPublicEvent(
  slug: string,
  src: string | undefined
): Promise<PublicEvent> {
  const client = createApiClient({
    baseUrl: `${serverApiBaseUrl()}${API_VERSION_PATH}`,
  })
  const result = await client.GET("/public/events/{slug}", {
    params: {
      path: { slug },
      query: src === "qr" ? { src: "qr" } : undefined,
    },
    cache: "force-cache",
    next: { revalidate: 60 },
  })
  return unwrapEnvelope<PublicEvent>(result)
}

export default async function PublicGalleryPage({
  params,
  searchParams,
}: {
  params: Promise<{ slug: string }>
  searchParams: Promise<{ src?: string }>
}) {
  const { slug } = await params
  const { src } = await searchParams

  let event: PublicEvent
  try {
    event = await fetchPublicEvent(slug, src)
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      notFound()
    }
    throw error
  }

  return <GalleryShell slug={slug} event={toGalleryEvent(event)} />
}
