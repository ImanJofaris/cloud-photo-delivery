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

test("event lifecycle: create, find, settings, archive, delete", async ({
  page,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)

  const email = `e2e-events-${Date.now()}@example.com`
  const eventName = `E2E Wedding ${Date.now()}`

  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })

  await page.goto("/events/new")
  await page.getByLabel("Event name").fill(eventName)
  await page.getByRole("button", { name: "Create event" }).click()
  await expect(page).toHaveURL(/\/events\/[0-9a-f-]+/, { timeout: 15_000 })
  await expect(page.getByRole("heading", { name: eventName })).toBeVisible()

  await page.goto("/events")
  await page.getByPlaceholder("Search events").fill(eventName)
  await expect(page.getByText(eventName)).toBeVisible({ timeout: 15_000 })

  await page.getByText(eventName).click()
  await expect(page).toHaveURL(/\/events\/[0-9a-f-]+/)

  await page.getByRole("tab", { name: "Settings" }).click()
  await page.getByRole("button", { name: "Save settings" }).click()
  await expect(page.getByText("Settings saved")).toBeVisible({
    timeout: 15_000,
  })

  await page.getByRole("tab", { name: "Danger zone" }).click()
  await page.getByRole("button", { name: "Archive", exact: true }).click()
  await expect(page.getByText("Event archived")).toBeVisible({
    timeout: 15_000,
  })

  await page.getByLabel("Event name").last().fill(eventName)
  await page.getByRole("button", { name: "Delete event" }).click()
  await expect(page).toHaveURL(/\/events$/, { timeout: 15_000 })
})
