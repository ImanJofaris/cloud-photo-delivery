"use client"

import * as React from "react"

export function useObjectUrl(blob: Blob | null): string | null {
  const url = React.useMemo(
    () => (blob ? URL.createObjectURL(blob) : null),
    [blob]
  )

  React.useEffect(() => {
    if (!url) return
    return () => URL.revokeObjectURL(url)
  }, [url])

  return url
}
