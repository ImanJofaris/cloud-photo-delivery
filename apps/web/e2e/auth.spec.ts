import { expect, test } from "@playwright/test"

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

test("redirects unauthenticated visitors to login", async ({ page }) => {
  await page.context().clearCookies()
  await page.goto("/dashboard")

  await expect(page).toHaveURL(/\/login/)
  await expect(page.getByLabel("Email")).toBeVisible()
  await expect(page.getByLabel("Password", { exact: true })).toBeVisible()
})

test("signs up, restores the session on reload, and logs out", async ({
  page,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)

  const email = `e2e-${Date.now()}@example.com`

  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()

  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()

  await page.reload()
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible({
    timeout: 15_000,
  })

  await page.getByRole("button", { name: email }).click()
  await page.getByRole("menuitem", { name: "Log out" }).click()

  await expect(page).toHaveURL(/\/login/, { timeout: 15_000 })
})
