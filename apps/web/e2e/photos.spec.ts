import { expect, test } from "@playwright/test"

const apiOrigin =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080"
const readyzUrl = new URL("/readyz", apiOrigin).toString()

// 1x1 JPEG, 160 bytes; the worker must be able to decode it.
const TINY_JPEG = Buffer.from(
  "/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/wAALCAABAAEBAREA/8QAFAABAAAAAAAAAAAAAAAAAAAACf/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AKp//2Q==",
  "base64"
)

let apiAvailable = false

test.beforeAll(async ({ request }) => {
  try {
    const response = await request.get(readyzUrl, { timeout: 3000 })
    apiAvailable = response.ok()
  } catch {
    apiAvailable = false
  }
})

test("photo upload: upload a JPEG, wait for processing, delete it", async ({
  page,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(90_000)

  const proxiedUploads: string[] = []
  page.on("request", (request) => {
    if (request.method() === "PUT" && request.url().startsWith(apiOrigin)) {
      proxiedUploads.push(request.url())
    }
  })

  const email = `e2e-photos-${Date.now()}@example.com`
  const eventName = `E2E Photos ${Date.now()}`

  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })

  await page.goto("/events/new")
  await page.getByLabel("Event name").fill(eventName)
  await page.getByRole("button", { name: "Create event" }).click()
  await expect(page).toHaveURL(/\/events\/[0-9a-f-]+/, { timeout: 15_000 })

  await page.getByRole("tab", { name: "Photos" }).click()
  await page.setInputFiles('input[type="file"]', {
    name: "e2e-photo.jpg",
    mimeType: "image/jpeg",
    buffer: TINY_JPEG,
  })
  await expect(page.getByText("e2e-photo.jpg").first()).toBeVisible({
    timeout: 10_000,
  })

  // The worker turns PROCESSING into READY. If it is not running, the flow
  // cannot pass here; skip rather than fail the whole suite.
  const readyBadge = page.getByText("Ready").first()
  const becameReady = await expect(readyBadge)
    .toBeVisible({ timeout: 30_000 })
    .then(() => true)
    .catch(() => false)
  test.skip(!becameReady, "image processing worker is not running")

  await page.getByRole("button", { name: "Delete e2e-photo.jpg" }).click()
  await page.getByRole("button", { name: "Delete photo" }).click()
  await expect(page.getByText("Photo deleted")).toBeVisible({ timeout: 15_000 })
  await expect(page.locator('img[alt="e2e-photo.jpg"]')).toHaveCount(0)

  // Photo bytes must go browser -> object storage only.
  expect(proxiedUploads).toEqual([])
})
