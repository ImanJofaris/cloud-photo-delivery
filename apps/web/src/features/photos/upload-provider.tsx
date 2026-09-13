"use client"

import { useQueryClient } from "@tanstack/react-query"
import * as React from "react"

import { eventKeys } from "@/features/events/keys"

import { createUploadDeps } from "./api"
import { photoKeys } from "./keys"
import { UploadQueue, type UploadItem } from "./uploader"

const UploadQueueContext = React.createContext<UploadQueue | null>(null)

export function UploadProvider({
  eventId,
  children,
}: {
  eventId: string
  children: React.ReactNode
}) {
  const queryClient = useQueryClient()
  const [queue] = React.useState(
    () =>
      new UploadQueue(eventId, {
        ...createUploadDeps(eventId),
        onChanged: () => {
          void queryClient.invalidateQueries({ queryKey: photoKeys.lists() })
          void queryClient.invalidateQueries({
            queryKey: eventKeys.dashboard(eventId),
          })
        },
      })
  )

  React.useEffect(() => {
    void queue.reconcile()
    return () => queue.abortAll()
  }, [queue])

  return (
    <UploadQueueContext.Provider value={queue}>
      {children}
    </UploadQueueContext.Provider>
  )
}

export function useUploadQueue(): UploadQueue {
  const queue = React.useContext(UploadQueueContext)
  if (!queue) {
    throw new Error("useUploadQueue must be used within an UploadProvider")
  }
  return queue
}

export function useUploadItems(queue: UploadQueue): UploadItem[] {
  return React.useSyncExternalStore(
    queue.subscribe,
    queue.snapshot,
    queue.snapshot
  )
}
