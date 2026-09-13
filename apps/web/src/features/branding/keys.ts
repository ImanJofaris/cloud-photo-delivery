export const brandingKeys = {
  all: ["branding"] as const,
  detail: () => [...brandingKeys.all, "detail"] as const,
}
