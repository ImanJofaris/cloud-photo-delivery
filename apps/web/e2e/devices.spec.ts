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
  const email = `e2e-devices-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })
}

test("device lifecycle: create, save key, rotate, revoke", async ({ page }) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(90_000)

  await signup(page)
  await page.goto("/devices")
  await expect(page.getByRole("heading", { name: "Devices" })).toBeVisible()
  await expect(page.getByText("No devices yet")).toBeVisible()

  await page.getByRole("button", { name: "Add device" }).click()
  await page.getByLabel("Device name").fill("E2E Booth")
  await page.getByRole("button", { name: "Create device" }).click()

  await expect(page.getByText("Device created")).toBeVisible({
    timeout: 15_000,
  })
  const keyValue = page.locator("#device-api-key")
  await expect(keyValue).toContainText("•")

  await page.getByRole("button", { name: "Reveal key" }).click()
  await expect(keyValue).toContainText(/^cpd_live_/)
  await expect(keyValue).not.toContainText("•")

  await page.getByRole("button", { name: "I've saved it" }).click()
  await expect(page.getByText("E2E Booth")).toBeVisible()

  await page.getByRole("button", { name: "Actions for E2E Booth" }).click()
  await page.getByRole("menuitem", { name: "Rotate key" }).click()
  await page.getByRole("button", { name: "Rotate key" }).click()
  await expect(page.getByText("Key rotated")).toBeVisible({
    timeout: 15_000,
  })
  await page.getByRole("button", { name: "I've saved it" }).click()

  await page.getByRole("button", { name: "Actions for E2E Booth" }).click()
  await page.getByRole("menuitem", { name: "Revoke", exact: true }).click()
  await page.getByRole("button", { name: "Revoke device" }).click()
  await expect(page.getByText("Revoked", { exact: true })).toBeVisible({
    timeout: 15_000,
  })

  await page.reload()
  await expect(page.getByText("Revoked", { exact: true })).toBeVisible({
    timeout: 15_000,
  })
  await expect(
    page.getByRole("button", { name: "Actions for E2E Booth" })
  ).toBeDisabled()
})
