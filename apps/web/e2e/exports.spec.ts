import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test"

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

async function signup(page: Page): Promise<string> {
  const email = `e2e-export-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })
  return email
}

async function createEvent(page: Page, name: string): Promise<string> {
  await page.goto("/events/new")
  await page.getByLabel("Event name").fill(name)
  await page.getByRole("button", { name: "Create event" }).click()
  await expect(page).toHaveURL(/\/events\/[0-9a-f-]+/, { timeout: 15_000 })
  const match = page.url().match(/\/events\/([0-9a-f-]+)/)
  if (!match) throw new Error("event id not found in the URL")
  return match[1]
}

async function accessToken(
  request: APIRequestContext,
  email: string
): Promise<string> {
  const login = await request.post(`${apiOrigin}/api/v1/auth/login`, {
    data: { email, password: "password123" },
  })
  const payload = (await login.json()) as { data?: { accessToken?: string } }
  const token = payload.data?.accessToken
  if (!token) throw new Error("could not obtain an access token")
  return token
}

test("zip export: request, poll to ready, download the signed URL", async ({
  page,
  request,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(150_000)

  const email = await signup(page)
  const eventId = await createEvent(page, `E2E Export ${Date.now()}`)

  await page.getByRole("tab", { name: "Photos" }).click()
  await page.setInputFiles('input[type="file"]', {
    name: "e2e-export.jpg",
    mimeType: "image/jpeg",
    buffer: TINY_JPEG,
  })
  await expect(page.getByText("e2e-export.jpg").first()).toBeVisible({
    timeout: 10_000,
  })

  const becameReady = await expect(page.getByText("Ready").first())
    .toBeVisible({ timeout: 30_000 })
    .then(() => true)
    .catch(() => false)
  test.skip(!becameReady, "image processing worker is not running")

  const token = await accessToken(request, email)
  const auth = { Authorization: `Bearer ${token}` }

  // Two immediate requests must dedupe onto the same active export.
  const first = await request.post(
    `${apiOrigin}/api/v1/events/${eventId}/exports`,
    { headers: auth }
  )
  expect(first.status()).toBe(202)
  const firstBody = (await first.json()) as { data?: { id?: string } }
  const second = await request.post(
    `${apiOrigin}/api/v1/events/${eventId}/exports`,
    { headers: auth }
  )
  const secondBody = (await second.json()) as { data?: { id?: string } }
  expect(secondBody.data?.id).toBe(firstBody.data?.id)

  await page.getByRole("tab", { name: "Overview" }).click()
  await page.getByRole("button", { name: /export all photos/i }).click()

  const downloadButton = page.getByRole("button", { name: /download zip/i })
  await expect(downloadButton).toBeVisible({ timeout: 90_000 })
  const href = await downloadButton.getAttribute("href")
  expect(href).toMatch(/^https?:\/\//)

  const downloadPromise = page.waitForEvent("download", { timeout: 30_000 })
  await downloadButton.click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toMatch(/\.zip$/i)
})
