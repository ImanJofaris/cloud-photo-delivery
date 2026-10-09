"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { Eye, EyeOff, Lock, TriangleAlert } from "lucide-react"
import * as React from "react"
import { Controller, useForm, useWatch } from "react-hook-form"
import { toast } from "sonner"

import { Alert, AlertDescription } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"
import { Switch } from "@workspace/ui/components/switch"
import {
  ToggleGroup,
  ToggleGroupItem,
} from "@workspace/ui/components/toggle-group"

import { useBranding } from "@/features/branding/api"
import { useSubscription } from "@/features/billing/api"

import {
  toUpdateEventSettingsRequest,
  useEvent,
  useEventSettings,
  useUpdateEventSettings,
} from "./api"
import { eventErrorMessage } from "./errors"
import { GuestPreviewCard } from "./guest-preview"
import { createEventSettingsSchema, type EventSettingsValues } from "./schema"

const VISIBILITY_OPTIONS: {
  value: EventSettingsValues["visibility"]
  label: string
}[] = [
  { value: "public", label: "Public" },
  { value: "password", label: "Password" },
  { value: "private", label: "Private" },
]

const TOGGLES = [
  {
    name: "allowDownload",
    label: "Allow downloads",
    description: "Guests can save web-size photos.",
  },
  {
    name: "allowOriginalDownload",
    label: "Original downloads",
    description:
      "Full resolution files, uses more guest data and plan storage traffic.",
  },
  {
    name: "watermarkEnabled",
    label: "Watermark",
    description: "Applies your branding watermark to guest previews.",
  },
] as const

const PASSWORD_FIELD_ID = "gallery-password"
const PASSWORD_HINT_ID = "gallery-password-hint"

export function EventSettingsForm({
  eventId,
  defaults,
}: {
  eventId: string
  defaults: EventSettingsValues
}) {
  const updateSettings = useUpdateEventSettings(eventId)
  const settingsQuery = useEventSettings(eventId)
  const eventQuery = useEvent(eventId)
  const brandingQuery = useBranding()
  const subscriptionQuery = useSubscription()
  const [showPassword, setShowPassword] = React.useState(false)

  const passwordProtected = settingsQuery.data?.passwordProtected ?? false
  const resolverSchema = React.useMemo(
    () => createEventSettingsSchema(passwordProtected),
    [passwordProtected]
  )
  const form = useForm<EventSettingsValues>({
    resolver: zodResolver(resolverSchema),
    defaultValues: defaults,
  })

  async function onSubmit(values: EventSettingsValues) {
    try {
      await updateSettings.mutateAsync(toUpdateEventSettingsRequest(values))
      form.setValue("password", "")
      toast.success("Settings saved")
    } catch (error) {
      form.setError("root", { message: eventErrorMessage(error) })
    }
  }

  const { errors, isSubmitting } = form.formState
  const visibility = useWatch({ control: form.control, name: "visibility" })
  const allowDownload = useWatch({
    control: form.control,
    name: "allowDownload",
  })
  const allowOriginalDownload = useWatch({
    control: form.control,
    name: "allowOriginalDownload",
  })
  const watermarkEnabled = useWatch({
    control: form.control,
    name: "watermarkEnabled",
  })

  const event = eventQuery.data
  const planAllowsBranding =
    subscriptionQuery.data?.plan.limits.branding === true
  const primaryColor = planAllowsBranding
    ? (brandingQuery.data?.primaryColor ?? null)
    : null
  const businessName = planAllowsBranding
    ? (brandingQuery.data?.businessName ?? null)
    : null

  const passwordEnabled = visibility === "password"
  const passwordHint = !passwordEnabled
    ? "Only needed for password protected events."
    : passwordProtected
      ? "Guests enter this before they see any photos. Leave blank to keep the current password."
      : "Guests enter this before they see any photos."

  return (
    <form
      onSubmit={form.handleSubmit(onSubmit)}
      className="space-y-4"
      noValidate
    >
      <div className="grid gap-4 lg:grid-cols-[1.35fr_1fr] lg:items-start">
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Gallery access</CardTitle>
              <CardAction>
                <Controller
                  control={form.control}
                  name="visibility"
                  render={({ field }) => (
                    <ToggleGroup
                      aria-label="Gallery visibility"
                      size="sm"
                      className="rounded-lg bg-muted p-0.5"
                      value={[field.value]}
                      onValueChange={(next) => {
                        const value = next.at(-1)
                        if (value) field.onChange(value)
                      }}
                    >
                      {VISIBILITY_OPTIONS.map((option) => (
                        <ToggleGroupItem
                          key={option.value}
                          value={option.value}
                          className="px-2.5 text-muted-foreground aria-pressed:bg-background! aria-pressed:text-foreground aria-pressed:shadow-sm"
                        >
                          {option.label}
                        </ToggleGroupItem>
                      ))}
                    </ToggleGroup>
                  )}
                />
              </CardAction>
            </CardHeader>
            <CardContent className="space-y-2">
              <Label htmlFor={PASSWORD_FIELD_ID}>Gallery password</Label>
              <div className="relative">
                <Lock
                  aria-hidden="true"
                  className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground"
                />
                <Input
                  id={PASSWORD_FIELD_ID}
                  type={passwordEnabled && showPassword ? "text" : "password"}
                  autoComplete="new-password"
                  className="pr-9 pl-8"
                  disabled={!passwordEnabled}
                  aria-invalid={Boolean(errors.password)}
                  aria-describedby={PASSWORD_HINT_ID}
                  placeholder={
                    passwordProtected
                      ? "Leave blank to keep the current password"
                      : "Enter a gallery password"
                  }
                  {...form.register("password")}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="absolute top-1/2 right-0.5 -translate-y-1/2"
                  disabled={!passwordEnabled}
                  aria-label={showPassword ? "Hide password" : "Show password"}
                  aria-pressed={showPassword}
                  onClick={() => setShowPassword((current) => !current)}
                >
                  {showPassword ? <EyeOff /> : <Eye />}
                </Button>
              </div>
              <p
                id={PASSWORD_HINT_ID}
                className="text-xs text-muted-foreground"
              >
                {passwordHint}
              </p>
              {errors.password && (
                <p className="text-sm text-destructive">
                  {errors.password.message}
                </p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Guest permissions</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="divide-y">
                {TOGGLES.map((toggle) => (
                  <div
                    key={toggle.name}
                    className="flex items-center justify-between gap-4 py-3 first:pt-1 last:pb-1"
                  >
                    <div className="min-w-0 flex-1">
                      <Label htmlFor={toggle.name} className="text-sm">
                        {toggle.label}
                      </Label>
                      <p className="text-xs text-muted-foreground">
                        {toggle.description}
                      </p>
                      {toggle.name === "allowOriginalDownload" &&
                      errors.allowOriginalDownload ? (
                        <p
                          role="alert"
                          className="mt-1 text-xs text-destructive"
                        >
                          {errors.allowOriginalDownload.message}
                        </p>
                      ) : null}
                    </div>
                    <Controller
                      control={form.control}
                      name={toggle.name}
                      render={({ field }) => (
                        <Switch
                          id={toggle.name}
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      )}
                    />
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="space-y-4">
          <GuestPreviewCard
            eventName={event?.name ?? ""}
            eventDate={event?.eventDate}
            location={event?.location}
            photoCount={event?.photoCount ?? 0}
            visibility={visibility}
            allowDownload={allowDownload}
            allowOriginalDownload={allowOriginalDownload}
            watermarkEnabled={watermarkEnabled}
            primaryColor={primaryColor}
            businessName={businessName}
          />

          <Card>
            <CardContent className="flex items-start gap-3">
              <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-amber-500/10 text-amber-700 dark:text-amber-400">
                <TriangleAlert className="size-4" />
              </div>
              <div>
                <p className="text-sm font-medium">
                  What guests see after expiry
                </p>
                <p className="text-xs text-muted-foreground">
                  When this event expires, guests see a not-found page instead
                  of the gallery. Extend it any time to reopen.
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      {errors.root && (
        <Alert variant="destructive">
          <AlertDescription>{errors.root.message}</AlertDescription>
        </Alert>
      )}

      <Button type="submit" disabled={isSubmitting}>
        {isSubmitting ? "Saving..." : "Save settings"}
      </Button>
    </form>
  )
}
