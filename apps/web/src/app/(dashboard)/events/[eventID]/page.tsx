import { EventDetail } from "@/features/events/event-detail"

export default async function EventPage({
  params,
}: {
  params: Promise<{ eventID: string }>
}) {
  const { eventID } = await params
  return <EventDetail eventId={eventID} />
}
