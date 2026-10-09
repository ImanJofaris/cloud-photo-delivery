"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import { Plus, Zap } from "lucide-react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import * as React from "react"
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
import { Separator } from "@workspace/ui/components/separator"
import { Switch } from "@workspace/ui/components/switch"
import { Textarea } from "@workspace/ui/components/textarea"

import { isPlanLimitReached } from "@/lib/api-errors"

import {
  applyEventSettings,
  toCreateEventRequest,
  toUpdateEventSettingsRequest,
  useCreateEvent,
} from "./api"
import { eventErrorMessage } from "./errors"
import { GuestPreviewCard } from "./guest-preview-card"
import { PlanUsageCard } from "./plan-usage-card"
import { newEventSchema, type NewEventValues } from "./schema"

export function EventForm() {
  const createEvent = useCreateEvent()
  const router = useRouter()
  const [planLimitReached, setPlanLimitReached] = React.useState(false)
  const form = useForm<NewEventValues>({
    resolver: zodResolver(newEventSchema),
    defaultValues: {
      name: "",
      eventDate: "",
      clientName: "",
      clientEmail: "",
      location: "",
      description: "",
      visibility: "public",
      password: "",
      allowDownload: true,
      allowOriginalDownload: false,
      watermarkEnabled: false,
    },
  })
  const values = useWatch({ control: form.control })

  const privateGallery = values.visibility === "private"
  const allowDownload = !privateGallery && Boolean(values.allowDownload)
  const allowOriginalDownload =
    allowDownload && Boolean(values.allowOriginalDownload)

  async function onSubmit(input: NewEventValues) {
    try {
      const result = await createEvent.mutateAsync(toCreateEventRequest(input))

      try {
        await applyEventSettings(
          result.event.id,
          toUpdateEventSettingsRequest({
            visibility: input.visibility,
            password:
              input.visibility === "password" ? input.password ?? "" : "",
            allowDownload:
              input.visibility === "private" ? false : input.allowDownload,
            allowOriginalDownload:
              input.visibility === "private" || !input.allowDownload
                ? false
                : input.allowOriginalDownload,
            watermarkEnabled: input.watermarkEnabled,
          })
        )
      } catch {
        toast.error(
          "Event created, but the gallery defaults could not be saved. You can change them in Settings."
        )
      }

      toast.success("Event created")
      router.push(`/events/${result.event.id}`)
    } catch (error) {
      setPlanLimitReached(isPlanLimitReached(error))
      form.setError("root", { message: eventErrorMessage(error) })
    }
  }

  const { errors, isSubmitting } = form.formState

  return (
    <div className="grid items-start gap-6 lg:grid-cols-[1.35fr_1fr]">
      <Card>
        <CardHeader>
          <CardTitle>Event details</CardTitle>
          <CardDescription>
            Set up the gallery, then upload photos or share the QR.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-5"
            noValidate
          >
            <div className="space-y-2">
              <Label htmlFor="name">Event name</Label>
              <Input
                id="name"
                placeholder="Sarah & John's Wedding"
                aria-invalid={Boolean(errors.name)}
                {...form.register("name")}
              />
              <p className="text-xs text-muted-foreground">
                Guests see this as the gallery title.
              </p>
              {errors.name && (
                <p className="text-sm text-destructive">{errors.name.message}</p>
              )}
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="eventDate">Event date</Label>
                <Input
                  id="eventDate"
                  type="date"
                  aria-invalid={Boolean(errors.eventDate)}
                  {...form.register("eventDate")}
                />
                <p className="text-xs text-muted-foreground">
                  Shown on the gallery header.
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="location">Location</Label>
                <Input
                  id="location"
                  placeholder="Kuala Lumpur"
                  aria-invalid={Boolean(errors.location)}
                  {...form.register("location")}
                />
                <p className="text-xs text-muted-foreground">
                  Venue or city, shown to guests.
                </p>
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="clientName">Client name</Label>
                <Input
                  id="clientName"
                  aria-invalid={Boolean(errors.clientName)}
                  {...form.register("clientName")}
                />
                <p className="text-xs text-muted-foreground">
                  Who the gallery is for.
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="clientEmail">Client email</Label>
                <Input
                  id="clientEmail"
                  type="email"
                  aria-invalid={Boolean(errors.clientEmail)}
                  {...form.register("clientEmail")}
                />
                <p className="text-xs text-muted-foreground">
                  Kept with the event for your records.
                </p>
                {errors.clientEmail && (
                  <p className="text-sm text-destructive">
                    {errors.clientEmail.message}
                  </p>
                )}
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">Description</Label>
              <Textarea
                id="description"
                rows={3}
                aria-invalid={Boolean(errors.description)}
                {...form.register("description")}
              />
              <p className="text-xs text-muted-foreground">
                Optional context under the gallery header.
              </p>
            </div>

            <Separator />

            <section className="space-y-4" aria-labelledby="gallery-defaults">
              <div>
                <p id="gallery-defaults" className="text-sm font-medium">
                  Gallery defaults
                </p>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Applied right after creation through the event settings
                  endpoint.
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="visibility">Visibility</Label>
                <Controller
                  control={form.control}
                  name="visibility"
                  render={({ field }) => (
                    <Select value={field.value} onValueChange={field.onChange}>
                      <SelectTrigger id="visibility" className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="public">
                          Public, anyone with the link
                        </SelectItem>
                        <SelectItem value="password">
                          Password protected
                        </SelectItem>
                        <SelectItem value="private">Private</SelectItem>
                      </SelectContent>
                    </Select>
                  )}
                />
              </div>

              {values.visibility === "password" && (
                <div className="space-y-2">
                  <Label htmlFor="password">Gallery password</Label>
                  <Input
                    id="password"
                    type="password"
                    autoComplete="new-password"
                    aria-invalid={Boolean(errors.password)}
                    {...form.register("password")}
                  />
                  <p className="text-xs text-muted-foreground">
                    Guests must enter this password to open the gallery.
                  </p>
                  {errors.password && (
                    <p className="text-sm text-destructive">
                      {errors.password.message}
                    </p>
                  )}
                </div>
              )}

              <div className="space-y-4">
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-0.5">
                    <Label htmlFor="allowDownload">Allow downloads</Label>
                    <p className="text-xs text-muted-foreground">
                      Guests can save web-size photos.
                    </p>
                  </div>
                  <Controller
                    control={form.control}
                    name="allowDownload"
                    render={({ field }) => (
                      <Switch
                        id="allowDownload"
                        checked={allowDownload}
                        disabled={privateGallery}
                        onCheckedChange={field.onChange}
                      />
                    )}
                  />
                </div>

                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-0.5">
                    <Label htmlFor="allowOriginalDownload">
                      Original downloads
                    </Label>
                    <p className="text-xs text-muted-foreground">
                      Full resolution, uses more data.
                    </p>
                  </div>
                  <Controller
                    control={form.control}
                    name="allowOriginalDownload"
                    render={({ field }) => (
                      <Switch
                        id="allowOriginalDownload"
                        checked={allowOriginalDownload}
                        disabled={privateGallery || !allowDownload}
                        onCheckedChange={field.onChange}
                      />
                    )}
                  />
                </div>
                {errors.allowOriginalDownload && (
                  <p className="text-sm text-destructive">
                    {errors.allowOriginalDownload.message}
                  </p>
                )}

                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-0.5">
                    <Label htmlFor="watermarkEnabled">Watermark previews</Label>
                    <p className="text-xs text-muted-foreground">
                      Show a watermark on gallery images.
                    </p>
                  </div>
                  <Controller
                    control={form.control}
                    name="watermarkEnabled"
                    render={({ field }) => (
                      <Switch
                        id="watermarkEnabled"
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    )}
                  />
                </div>
              </div>
            </section>

            {errors.root && (
              <Alert variant="destructive">
                <AlertDescription>
                  <p>{errors.root.message}</p>
                  {planLimitReached && (
                    <Button
                      render={<Link href="/billing" />}
                      nativeButton={false}
                      size="sm"
                      variant="outline"
                      className="mt-2"
                    >
                      View plans
                    </Button>
                  )}
                </AlertDescription>
              </Alert>
            )}

            <div className="flex flex-wrap items-center gap-3">
              <Button type="submit" disabled={isSubmitting}>
                <Plus />
                {isSubmitting ? "Creating..." : "Create event"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                render={<Link href="/events" />}
                nativeButton={false}
              >
                Cancel
              </Button>
              <span className="text-xs text-muted-foreground sm:ml-auto">
                Everything stays editable in Settings.
              </span>
            </div>
          </form>
        </CardContent>
      </Card>

      <div className="space-y-4">
        <GuestPreviewCard
          values={{
            name: values.name ?? "",
            eventDate: values.eventDate ?? "",
            location: values.location ?? "",
            visibility: values.visibility ?? "public",
            allowDownload,
            allowOriginalDownload,
            watermarkEnabled: Boolean(values.watermarkEnabled),
          }}
        />
        <PlanUsageCard />
        <Alert className="border-primary/20 bg-primary/5">
          <Zap aria-hidden="true" />
          <AlertDescription>
            <span className="font-medium text-foreground">Fast start</span>
            <span className="mt-0.5 block">
              After saving you land on the event page to upload photos and print
              the QR standee.
            </span>
          </AlertDescription>
        </Alert>
      </div>
    </div>
  )
}
