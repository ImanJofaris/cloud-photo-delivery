import { EventForm } from "@/features/events/event-form"

export default function NewEventPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">New event</h1>
        <p className="text-sm text-muted-foreground">
          Set up an event gallery in under a minute.
        </p>
      </div>
      <EventForm />
    </div>
  )
}
