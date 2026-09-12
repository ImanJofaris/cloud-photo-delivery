"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, useWatch } from "react-hook-form"
import { toast } from "sonner"

import { Alert, AlertDescription } from "@workspace/ui/components/alert"
import { Button } from "@workspace/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card"
import { Input } from "@workspace/ui/components/input"
import { Label } from "@workspace/ui/components/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select"
import { Switch } from "@workspace/ui/components/switch"

import { toUpdateEventSettingsRequest, useUpdateEventSettings } from "./api"
import { eventErrorMessage } from "./errors"
import { eventSettingsSchema, type EventSettingsValues } from "./schema"

const TOGGLES = [
  {
    name: "allowDownload",
    label: "Allow downloads",
    description: "Guests can download the processed (large) variant.",
  },
  {
    name: "allowOriginalDownload",
    label: "Allow original downloads",
    description: "Guests can download the untouched original file.",
  },
  {
    name: "watermarkEnabled",
    label: "Watermark previews",
    description: "Show a watermark on gallery images.",
  },
] as const

export function EventSettingsForm({
  eventId,
  defaults,
}: {
  eventId: string
  defaults: EventSettingsValues
}) {
  const updateSettings = useUpdateEventSettings(eventId)
  const form = useForm<EventSettingsValues>({
    resolver: zodResolver(eventSettingsSchema),
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

  return (
    <Card>
      <CardHeader>
        <CardTitle>Gallery settings</CardTitle>
        <CardDescription>
          Controls what guests can see and download.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className="space-y-5"
          noValidate
        >
          <div className="space-y-2">
            <Label htmlFor="visibility">Visibility</Label>
            <Controller
              control={form.control}
              name="visibility"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger id="visibility" className="w-56">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="public">Public</SelectItem>
                    <SelectItem value="password">Password protected</SelectItem>
                    <SelectItem value="private">Private</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />
          </div>

          {visibility === "password" && (
            <div className="space-y-2">
              <Label htmlFor="password">Gallery password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="new-password"
                placeholder="Leave blank to keep the current password"
                {...form.register("password")}
              />
              <p className="text-xs text-muted-foreground">
                Guests must enter this password to open the gallery.
              </p>
            </div>
          )}

          <div className="space-y-4">
            {TOGGLES.map((toggle) => (
              <div
                key={toggle.name}
                className="flex items-start justify-between gap-4"
              >
                <div className="space-y-0.5">
                  <Label htmlFor={toggle.name}>{toggle.label}</Label>
                  <p className="text-xs text-muted-foreground">
                    {toggle.description}
                  </p>
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
            {errors.allowOriginalDownload && (
              <p className="text-sm text-destructive">
                {errors.allowOriginalDownload.message}
              </p>
            )}
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
      </CardContent>
    </Card>
  )
}
