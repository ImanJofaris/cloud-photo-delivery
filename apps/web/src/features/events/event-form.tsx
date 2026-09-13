"use client"

import { zodResolver } from "@hookform/resolvers/zod"
import Link from "next/link"
import * as React from "react"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
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
import { Textarea } from "@workspace/ui/components/textarea"

import { isPlanLimitReached } from "@/lib/api-errors"

import { toCreateEventRequest, useCreateEvent } from "./api"
import { eventErrorMessage } from "./errors"
import { createEventSchema, type CreateEventValues } from "./schema"

export function EventForm() {
  const createEvent = useCreateEvent()
  const router = useRouter()
  const [planLimitReached, setPlanLimitReached] = React.useState(false)
  const form = useForm<CreateEventValues>({
    resolver: zodResolver(createEventSchema),
    defaultValues: {
      name: "",
      eventDate: "",
      clientName: "",
      clientEmail: "",
      location: "",
      description: "",
    },
  })

  async function onSubmit(values: CreateEventValues) {
    try {
      const result = await createEvent.mutateAsync(toCreateEventRequest(values))
      toast.success("Event created")
      router.push(`/events/${result.event.id}`)
    } catch (error) {
      setPlanLimitReached(isPlanLimitReached(error))
      form.setError("root", { message: eventErrorMessage(error) })
    }
  }

  const { errors, isSubmitting } = form.formState

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>New event</CardTitle>
        <CardDescription>
          Create the event, then upload photos from a photobooth or the
          dashboard.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className="space-y-4"
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
            </div>
            <div className="space-y-2">
              <Label htmlFor="location">Location</Label>
              <Input
                id="location"
                placeholder="Kuala Lumpur"
                aria-invalid={Boolean(errors.location)}
                {...form.register("location")}
              />
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
            </div>
            <div className="space-y-2">
              <Label htmlFor="clientEmail">Client email</Label>
              <Input
                id="clientEmail"
                type="email"
                aria-invalid={Boolean(errors.clientEmail)}
                {...form.register("clientEmail")}
              />
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
          </div>

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

          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? "Creating..." : "Create event"}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
