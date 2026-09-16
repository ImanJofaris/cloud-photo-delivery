export const adminKeys = {
  all: ["admin"] as const,
  stats: () => [...adminKeys.all, "stats"] as const,
  users: () => [...adminKeys.all, "users"] as const,
  subscriptions: () => [...adminKeys.all, "subscriptions"] as const,
  health: () => [...adminKeys.all, "health"] as const,
}
