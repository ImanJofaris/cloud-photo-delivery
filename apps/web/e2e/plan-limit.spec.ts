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
    data: { name: `E2E Active ${Date.now()}`, status: "active" },
  })
  expect(created.ok()).toBe(true)

  // The F2 create form does not expose a status field (events default to
  // upcoming, which does not count against the active-event limit), so mark
  // the UI request active to exercise the real 402 from the API.
  await page.route("**/api/v1/events", async (route) => {
    if (route.request().method() !== "POST") {
      await route.continue()
      return
    }
    const body = route.request().postDataJSON() as Record<string, unknown>
    await route.continue({
      postData: JSON.stringify({ ...body, status: "active" }),
    })
  })

  await page.goto("/events/new")
  await page.getByLabel("Event name").fill(`E2E Blocked ${Date.now()}`)
  await page.getByRole("button", { name: "Create event" }).click()

  await expect(
    page.getByText("You have reached your plan's active event limit.")
  ).toBeVisible({ timeout: 15_000 })

  const billingLink = page.getByRole("button", { name: "View plans" })
  await expect(billingLink).toHaveAttribute("href", "/billing")
  await billingLink.click()
  await expect(page).toHaveURL(/\/billing/)
})
