export const accountKeys = {
  all: ["account"] as const,
  profile: () => [...accountKeys.all, "me"] as const,
}
