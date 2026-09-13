export class UploadTransportError extends Error {
  readonly status: number

  constructor(status: number, message?: string) {
    super(message ?? `Upload failed with status ${status}`)
    this.name = "UploadTransportError"
    this.status = status
  }
}

export type PutInput = {
  url: string
  body: Blob
  onProgress?: (loaded: number, total: number) => void
  signal?: AbortSignal
}

export type PutResult = { etag: string | null }

export type PutTransport = (input: PutInput) => Promise<PutResult>

// XHR is required here because fetch cannot report upload progress.
export const putWithProgress: PutTransport = ({
  url,
  body,
  onProgress,
  signal,
}) =>
  new Promise<PutResult>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open("PUT", url)
    if (body.type) {
      xhr.setRequestHeader("Content-Type", body.type)
    }

    const cleanup = () => {
      signal?.removeEventListener("abort", onAbort)
    }
    const onAbort = () => xhr.abort()

    xhr.upload.onprogress = (event) => {
      onProgress?.(
        event.loaded,
        event.lengthComputable ? event.total : body.size
      )
    }
    xhr.onload = () => {
      cleanup()
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve({ etag: xhr.getResponseHeader("ETag") })
      } else {
        reject(new UploadTransportError(xhr.status))
      }
    }
    xhr.onerror = () => {
      cleanup()
      reject(new UploadTransportError(0, "Network error while uploading."))
    }
    xhr.ontimeout = () => {
      cleanup()
      reject(new UploadTransportError(0, "The upload timed out."))
    }
    xhr.onabort = () => {
      cleanup()
      reject(new DOMException("Aborted", "AbortError"))
    }

    signal?.addEventListener("abort", onAbort, { once: true })
    xhr.send(body)
  })
