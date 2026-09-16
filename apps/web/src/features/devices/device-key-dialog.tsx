"use client"

import { Check, Copy, Eye, EyeOff, ShieldAlert } from "lucide-react"
import * as React from "react"
import { toast } from "sonner"

import type { components } from "@workspace/api-client"

import { Alert, AlertDescription } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@workspace/ui/components/dialog"
import { Label } from "@workspace/ui/components/label"

type DeviceWithKey = components["schemas"]["DeviceWithKey"]

export function maskApiKey(key: string): string {
  if (key.length <= 8) return "•".repeat(key.length)
  return `${key.slice(0, 4)}${"•".repeat(16)}${key.slice(-4)}`
}

export function DeviceKeyDialog({
  result,
  action,
  onClose,
}: {
  result: DeviceWithKey | null
  action: "created" | "rotated"
  onClose: () => void
}) {
  const [revealed, setRevealed] = React.useState(false)
  const [copied, setCopied] = React.useState(false)

  const key = result?.key ?? ""

  function close() {
    setRevealed(false)
    setCopied(false)
    onClose()
  }

  async function copyKey() {
    try {
      await navigator.clipboard.writeText(key)
      setCopied(true)
      toast.success("API key copied")
    } catch {
      toast.error("Could not copy the key")
    }
  }

  return (
    <Dialog open={result !== null} onOpenChange={() => {}} disablePointerDismissal>
      <DialogContent showCloseButton={false} className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {action === "created" ? "Device created" : "Key rotated"}
          </DialogTitle>
          <DialogDescription>
            Save this key now. It cannot be shown again.
          </DialogDescription>
        </DialogHeader>

        <Alert>
          <ShieldAlert />
          <AlertDescription>
            This is the only time the API key appears. If you lose it, rotate
            the key to issue a new one.
          </AlertDescription>
        </Alert>

        <div className="space-y-2">
          <Label htmlFor="device-api-key">API key</Label>
          <div className="flex items-center gap-2">
            <code
              id="device-api-key"
              className="min-w-0 flex-1 truncate rounded-md bg-muted px-3 py-2 font-mono text-xs"
            >
              {revealed ? key : maskApiKey(key)}
            </code>
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label={revealed ? "Hide key" : "Reveal key"}
              onClick={() => setRevealed((value) => !value)}
            >
              {revealed ? <EyeOff /> : <Eye />}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label="Copy key"
              onClick={() => void copyKey()}
            >
              {copied ? <Check /> : <Copy />}
            </Button>
          </div>
        </div>

        <div className="min-w-0 space-y-2">
          <p className="text-sm text-muted-foreground">
            Devices authenticate uploads with the{" "}
            <code className="font-mono">X-Api-Key</code> header, for example:
          </p>
          <pre className="overflow-x-auto rounded-md bg-muted p-3 text-xs">
            {`curl -X POST /api/v1/events/{eventId}/uploads \\\n  -H "X-Api-Key: ${
              revealed ? key : "••••••••"
            }" \\\n  -H "Content-Type: application/json" \\\n  -d '{"filename":"photo.jpg","contentType":"image/jpeg","size":1024}'`}
          </pre>
        </div>

        <DialogFooter>
          <Button type="button" onClick={close}>
            I&apos;ve saved it
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
