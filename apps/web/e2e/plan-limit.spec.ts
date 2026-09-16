import { expect, test, type Page } from "@playwright/test"

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
  const email = `e2e-limit-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })
  return email
}

test("plan limit: the 402 links to billing instead of dead-ending", async ({
  page,
  request,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(90_000)

  const email = await signup(page)

  const login = await request.post(`${apiOrigin}/api/v1/auth/login`, {
    data: { email, password: "password123" },
  })
  const payload = (await login.json()) as {
    data?: { accessToken?: string }
  }
  const token = payload.data?.accessToken
  if (!token) throw new Error("could not obtain an access token")

  const created = await request.post(`${apiOrigin}/api/v1/events`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { name: `E2E Active ${Date.now()}` },
  })
  expect(created.ok()).toBe(true)

  // Every event guests can reach occupies a Free slot, so the second create
  // is blocked with the real 402 and renders the upgrade link.
  await page.goto("/events/new")
  await page.getByLabel("Event name").fill(`E2E Blocked ${Date.now()}`)
  await page.getByRole("button", { name: "Create event" }).click()

  await expect(
    page.getByText("You have reached your plan's event limit.")
  ).toBeVisible({ timeout: 15_000 })

  const billingLink = page.getByRole("button", { name: "View plans" })
  await expect(billingLink).toHaveAttribute("href", "/billing")
  await billingLink.click()
  await expect(page).toHaveURL(/\/billing/)
})
