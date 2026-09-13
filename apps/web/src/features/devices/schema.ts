import { z } from "zod"

import type { components } from "@workspace/api-client"

export const MAX_DEVICE_NAME_LENGTH = 120
export const NO_EVENT = "no-event"

export const deviceNameSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Device name is required.")
    .max(
      MAX_DEVICE_NAME_LENGTH,
      `Device name must be ${MAX_DEVICE_NAME_LENGTH} characters or fewer.`
    ),
})

export type DeviceNameValues = z.infer<typeof deviceNameSchema>

export const createDeviceSchema = deviceNameSchema.extend({
  assignedEventId: z.string(),
})

export type CreateDeviceValues = z.infer<typeof createDeviceSchema>

export function toCreateDeviceRequest(
  values: CreateDeviceValues
): components["schemas"]["CreateDeviceRequest"] {
  const parsed = createDeviceSchema.parse(values)
  return {
    name: parsed.name,
    ...(parsed.assignedEventId === NO_EVENT
      ? {}
      : { assignedEventId: parsed.assignedEventId }),
  }
}
