export const analyticsKeys = {
  all: ["analytics"] as const,
  account: (days: number) => [...analyticsKeys.all, "account", days] as const,
  event: (eventId: string, days: number) =>
    [...analyticsKeys.all, "event", eventId, days] as const,
}
