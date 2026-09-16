import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test"

const apiOrigin =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080"
const readyzUrl = new URL("/readyz", apiOrigin).toString()
const DAY_MS = 86_400_000

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
  const email = `e2e-lifecycle-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
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

async function seedEvent(
  request: APIRequestContext,
  auth: Record<string, string>,
  name: string,
  extra: Record<string, unknown>
): Promise<{ id: string; name: string; status: string }> {
  const response = await request.post(`${apiOrigin}/api/v1/events`, {
    headers: auth,
    data: { name, status: "upcoming", ...extra },
  })
  expect(response.status()).toBe(201)
  const payload = (await response.json()) as {
    data?: { event?: { id: string; name: string; status: string } }
  }
  const event = payload.data?.event
  if (!event) throw new Error("event was not created")
  return event
}

test("lifecycle: expired banner, extend, expiring-soon hint, expired filter", async ({
  page,
  request,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(120_000)

  const email = await signup(page)
  const token = await accessToken(request, email)
  const auth = { Authorization: `Bearer ${token}` }
  const prefix = `E2E Lifecycle ${Date.now()}`

  const soonName = `${prefix} Soon`
  const expiredName = `${prefix} Expired`
  const filteredName = `${prefix} Filter`

  await seedEvent(request, auth, soonName, {
    expiresAt: new Date(Date.now() + 3 * DAY_MS - 3_600_000).toISOString(),
  })
  const expired = await seedEvent(request, auth, expiredName, {
    status: "expired",
    expiresAt: new Date(Date.now() - 2 * DAY_MS).toISOString(),
  })
  await seedEvent(request, auth, filteredName, {
    status: "expired",
    expiresAt: new Date(Date.now() - 4 * DAY_MS).toISOString(),
  })

  await page.goto(`/events/${expired.id}`)
  await expect(page.getByText(/this event expired/i).first()).toBeVisible({
    timeout: 15_000,
  })
  await expect(page.getByText(/gallery is no longer available/i)).toBeVisible()

  await page.getByRole("button", { name: "Extend" }).first().click()
  await page.getByRole("button", { name: "30 days" }).click()
  await page.getByRole("button", { name: "Extend event" }).click()
  await expect(page.getByText(/event extended to/i)).toBeVisible({
    timeout: 15_000,
  })
  await expect(page.getByText(/this event expired/i)).toHaveCount(0)
  await expect(page.locator('[data-slot="badge"]').first()).toHaveText("active")

  await page.goto("/events")
  await page.getByPlaceholder("Search events").fill(prefix)
  await expect(page.getByText(soonName)).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText("Expires in 3 days")).toBeVisible()

  await page.getByRole("combobox").click()
  await page.getByRole("option", { name: "Expired" }).click()

  await expect(page.getByText(filteredName)).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText(soonName)).toHaveCount(0)
  await expect(page.getByText(expiredName)).toHaveCount(0)
})
