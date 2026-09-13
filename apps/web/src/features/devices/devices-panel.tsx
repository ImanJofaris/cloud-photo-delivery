"use client"

import { Camera, MoreHorizontal, Plus } from "lucide-react"
import * as React from "react"

import type { components } from "@workspace/api-client"

import { Badge } from "@workspace/ui/components/badge"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@workspace/ui/components/dropdown-menu"
import { Skeleton } from "@workspace/ui/components/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@workspace/ui/components/table"

import { useEventOptions } from "@/features/events/api"

import { useDevices } from "./api"
import { DeviceFormDialog } from "./device-form-dialog"
import { DeviceKeyDialog } from "./device-key-dialog"
import { formatDeviceTimestamp, resolveEventName, UNKNOWN_EVENT } from "./format"
import { RenameDeviceDialog } from "./rename-device-dialog"
import { RevokeDeviceDialog } from "./revoke-device-dialog"
import { RotateKeyDialog } from "./rotate-key-dialog"

type Device = components["schemas"]["Device"]
type DeviceWithKey = components["schemas"]["DeviceWithKey"]

type KeyResult = {
  result: DeviceWithKey
  action: "created" | "rotated"
}

export function DevicesPanel() {
  const devicesQuery = useDevices()
  const eventsQuery = useEventOptions()

  const [createOpen, setCreateOpen] = React.useState(false)
  const [renameTarget, setRenameTarget] = React.useState<Device | null>(null)
  const [rotateTarget, setRotateTarget] = React.useState<Device | null>(null)
  const [revokeTarget, setRevokeTarget] = React.useState<Device | null>(null)
  const [keyResult, setKeyResult] = React.useState<KeyResult | null>(null)

  const events = React.useMemo(
    () => eventsQuery.data ?? [],
    [eventsQuery.data]
  )
  const activeEvents = React.useMemo(
    () => events.filter((event) => event.status === "active"),
    [events]
  )
  const eventNames = React.useMemo(
    () => new Map(events.map((event) => [event.id, event.name])),
    [events]
  )

  const devices = devicesQuery.data?.items ?? []

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">Devices</h1>
          <p className="text-sm text-muted-foreground">
            Photobooths that upload straight to your events with an API key.
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus />
          Add device
        </Button>
      </div>

      {devicesQuery.isPending && (
        <div className="space-y-3">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      )}

      {devicesQuery.isError && (
        <Card>
          <CardContent className="py-8 text-sm text-destructive">
            Could not load devices. Please try again.
          </CardContent>
        </Card>
      )}

      {!devicesQuery.isPending &&
        !devicesQuery.isError &&
        devices.length === 0 && (
          <Card>
            <CardContent className="flex flex-col items-center gap-3 py-12 text-center">
              <Camera className="size-8 text-muted-foreground" />
              <div>
                <p className="font-medium">No devices yet</p>
                <p className="max-w-md text-sm text-muted-foreground">
                  Device keys upload only to their assigned event and are
                  limited to 100 uploads per minute per device.
                </p>
              </div>
            </CardContent>
          </Card>
        )}

      {devices.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>All devices</CardTitle>
            <CardDescription>
              Revoked devices stay listed for audit.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Key prefix</TableHead>
                  <TableHead>Event</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Last used</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {devices.map((device) => {
                  const revoked = Boolean(device.revokedAt)
                  const eventName = resolveEventName(
                    device.assignedEventId,
                    eventNames
                  )
                  return (
                    <TableRow
                      key={device.id}
                      className={revoked ? "text-muted-foreground" : undefined}
                    >
                      <TableCell className="font-medium">
                        {device.name}
                      </TableCell>
                      <TableCell className="font-mono text-xs">
                        {device.keyPrefix}
                      </TableCell>
                      <TableCell>
                        {eventName === null ? (
                          <span className="text-muted-foreground">
                            Unscoped
                          </span>
                        ) : eventName === UNKNOWN_EVENT ? (
                          <span className="text-muted-foreground">
                            {UNKNOWN_EVENT}
                          </span>
                        ) : (
                          eventName
                        )}
                      </TableCell>
                      <TableCell>
                        {revoked ? (
                          <Badge variant="outline">Revoked</Badge>
                        ) : (
                          <Badge variant="secondary">Active</Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        {formatDeviceTimestamp(device.lastUsedAt)}
                      </TableCell>
                      <TableCell>
                        {formatDeviceTimestamp(device.createdAt)}
                      </TableCell>
                      <TableCell className="text-right">
                        <DropdownMenu>
                          <DropdownMenuTrigger
                            disabled={revoked}
                            render={
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                aria-label={`Actions for ${device.name}`}
                              >
                                <MoreHorizontal />
                              </Button>
                            }
                          />
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem
                              onClick={() => setRenameTarget(device)}
                            >
                              Rename
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => setRotateTarget(device)}
                            >
                              Rotate key
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              variant="destructive"
                              onClick={() => setRevokeTarget(device)}
                            >
                              Revoke
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      <DeviceFormDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        events={activeEvents}
        onCreated={(result) =>
          setKeyResult({ result, action: "created" })
        }
      />

      <DeviceKeyDialog
        result={keyResult?.result ?? null}
        action={keyResult?.action ?? "created"}
        onClose={() => setKeyResult(null)}
      />

      <RenameDeviceDialog
        device={renameTarget}
        onClose={() => setRenameTarget(null)}
      />

      <RotateKeyDialog
        device={rotateTarget}
        onClose={() => setRotateTarget(null)}
        onRotated={(result) => setKeyResult({ result, action: "rotated" })}
      />

      <RevokeDeviceDialog
        device={revokeTarget}
        onClose={() => setRevokeTarget(null)}
      />
    </div>
  )
}
