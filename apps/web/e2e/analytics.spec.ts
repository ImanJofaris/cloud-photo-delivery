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
  const email = `e2e-analytics-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })
  return email
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

async function counterValue(page: Page, label: string): Promise<number> {
  const card = page
    .locator('[data-slot="card"]')
    .filter({ hasText: label })
    .first()
  const text = await card.locator('[data-slot="card-title"]').innerText()
  return Number(text.replace(/[^0-9]/g, ""))
}

// The seeded event needs a 30-day expiry, which the Free plan's 7-day
// retention does not allow.
async function subscribe(
  request: APIRequestContext,
  email: string,
  planId: string
) {
  const login = await request.post(`${apiOrigin}/api/v1/auth/login`, {
    data: { email, password: "password123" },
  })
  const payload = (await login.json()) as { data?: { accessToken?: string } }
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

test("analytics: gallery views and QR scans reach the account and event views", async ({
  page,
  request,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(120_000)

  const email = await signup(page)
  await subscribe(request, email, "starter")
  const token = await accessToken(request, email)

  const name = `E2E Analytics ${Date.now()}`
  const created = await request.post(`${apiOrigin}/api/v1/events`, {
    headers: { Authorization: `Bearer ${token}` },
    data: {
      name,
      status: "active",
      expiresAt: new Date(Date.now() + 30 * 86_400_000).toISOString(),
    },
  })
  expect(created.status()).toBe(201)
  const payload = (await created.json()) as {
    data?: { event?: { id: string; slug: string } }
  }
  const event = payload.data?.event
  if (!event) throw new Error("event was not created")

  await page.goto(`/e/${event.slug}?src=qr`)
  await expect(page.getByRole("heading", { name })).toBeVisible({
    timeout: 30_000,
  })
  // Let the session bootstrap refresh settle before the next cold load, so
  // two refreshes cannot race the rotating refresh token.
  await page.waitForLoadState("networkidle")

  await page.goto("/analytics")
  await expect(page.getByRole("heading", { name: "Analytics" })).toBeVisible({
    timeout: 15_000,
  })
  await expect
    .poll(() => counterValue(page, "Gallery views"), { timeout: 20_000 })
    .toBeGreaterThanOrEqual(1)
  await expect
    .poll(() => counterValue(page, "QR scans"), { timeout: 20_000 })
    .toBeGreaterThanOrEqual(1)

  await page.goto(`/events/${event.id}`)
  await expect(page.getByRole("heading", { name })).toBeVisible({
    timeout: 30_000,
  })
  await page.getByRole("tab", { name: "Analytics" }).click()
  await expect
    .poll(() => counterValue(page, "Gallery views"), { timeout: 20_000 })
    .toBeGreaterThanOrEqual(1)
  await expect
    .poll(() => counterValue(page, "QR scans"), { timeout: 20_000 })
    .toBeGreaterThanOrEqual(1)
})
