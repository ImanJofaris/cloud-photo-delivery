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

async function signup(page: Page) {
  const email = `e2e-billing-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })
}

test("billing: subscribe with manual billing, cancel, and resume", async ({
  page,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(90_000)

  await signup(page)
  await page.goto("/billing")
  await expect(page.getByRole("heading", { name: "Billing" })).toBeVisible()
  await expect(page.getByText("Free plan", { exact: true })).toBeVisible()

  await page.getByRole("button", { name: "Subscribe to Starter" }).click()
  await expect(
    page.getByText("Subscribe to Starter?")
  ).toBeVisible()
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Subscribe" })
    .click()

  await expect(page.getByText("Offline billing")).toBeVisible({
    timeout: 15_000,
  })
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Done" })
    .click()

  await expect(page.getByText(/Starter plan/)).toBeVisible({
    timeout: 15_000,
  })
  await expect(page.getByText("Active", { exact: true })).toBeVisible()
  await expect(page.getByText("Active events", { exact: true })).toBeVisible()

  await page.getByRole("button", { name: "Cancel subscription" }).click()
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Cancel subscription" })
    .click()

  await expect(page.getByText(/Cancels on/)).toBeVisible({ timeout: 15_000 })

  await page.getByRole("button", { name: "Resume" }).click()
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Resume subscription" })
    .click()

  await expect(page.getByText(/Renews on/)).toBeVisible({ timeout: 15_000 })
})
