import { z } from "zod"

export const createEventSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Event name is required")
    .max(255, "Event name is too long"),
  eventDate: z.string().optional().or(z.literal("")),
  clientName: z
    .string()
    .max(255, "Client name is too long")
    .optional()
    .or(z.literal("")),
  clientEmail: z
    .union([z.email("Enter a valid email"), z.literal("")])
    .optional(),
  location: z
    .string()
    .max(255, "Location is too long")
    .optional()
    .or(z.literal("")),
  description: z
    .string()
    .max(2000, "Description is too long")
    .optional()
    .or(z.literal("")),
})

export type CreateEventValues = z.infer<typeof createEventSchema>

export const eventSettingsSchema = z
  .object({
    visibility: z.enum(["public", "password", "private"]),
    password: z
      .string()
      .max(128, "Password is too long")
      .optional()
      .or(z.literal("")),
    allowDownload: z.boolean(),
    allowOriginalDownload: z.boolean(),
    watermarkEnabled: z.boolean(),
  })
  .refine((values) => values.allowDownload || !values.allowOriginalDownload, {
    message: "Enable downloads before allowing original downloads",
    path: ["allowOriginalDownload"],
  })

export type EventSettingsValues = z.infer<typeof eventSettingsSchema>

export const EXTEND_PRESET_DAYS = [7, 30, 90] as const
export const EXTEND_MIN_DAYS = 1
export const EXTEND_MAX_DAYS = 3650

export function parseExtendDays(input: string): number | null {
  const value = Number(input.trim())
  if (!Number.isInteger(value)) return null
  return value >= EXTEND_MIN_DAYS && value <= EXTEND_MAX_DAYS ? value : null
}
