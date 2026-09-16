import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test"

const apiOrigin =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080"
const readyzUrl = new URL("/readyz", apiOrigin).toString()

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
  const email = `e2e-branding-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })
  return email
}

// Custom branding is a Starter entitlement; the manual provider activates
// immediately, so subscribing over the API is enough for the UI to see it.
async function subscribe(
  request: APIRequestContext,
  email: string,
  planId: string
) {
  const login = await request.post(`${apiOrigin}/api/v1/auth/login`, {
    data: { email, password: "password123" },
  })
  const payload = (await login.json()) as {
    data?: { accessToken?: string }
  }
  const token = payload.data?.accessToken
  if (!token) throw new Error("could not obtain an access token")

  const res = await request.post(`${apiOrigin}/api/v1/billing/subscribe`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { planId },
  })
  if (!res.ok()) {
    throw new Error(`subscribe failed: ${res.status()} ${await res.text()}`)
  }
}

async function createEvent(page: Page, name: string): Promise<string> {
  await page.goto("/events/new")
  await page.getByLabel("Event name").fill(name)
  await page.getByRole("button", { name: "Create event" }).click()
  await expect(page).toHaveURL(/\/events\/[0-9a-f-]+/, { timeout: 15_000 })

  const link = page.getByRole("link", { name: "Open gallery" })
  const href = await link.getAttribute("href")
  if (!href) throw new Error("gallery link not found")
  return href
}

test("branding: save business name and color, then see them in the gallery", async ({
  page,
  request,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(90_000)

  const email = await signup(page)
  await subscribe(request, email, "starter")
  const eventName = `E2E Branding ${Date.now()}`
  const galleryHref = await createEvent(page, eventName)

  await page.goto("/branding")
  await page.getByLabel("Business name").fill("E2E Booth Branding")
  await page.getByLabel("Primary color", { exact: true }).fill("#ff0066")
  await page
    .getByLabel("Contact email")
    .fill("branding@example.com")
  await page.getByRole("button", { name: "Save branding" }).click()

  await expect(page.getByText("Branding saved")).toBeVisible({
    timeout: 15_000,
  })

  await page.goto(galleryHref)
  await expect(
    page.getByRole("heading", { name: eventName })
  ).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText("E2E Booth Branding")).toBeVisible({
    timeout: 15_000,
  })
  await expect(
    page.getByRole("link", { name: /branding@example\.com/ })
  ).toBeVisible()
})
