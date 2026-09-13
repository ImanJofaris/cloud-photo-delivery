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
  const email = `e2e-qr-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })
}

test.describe("event QR", () => {
  test.use({ permissions: ["clipboard-read", "clipboard-write"] })

  test("previews the blob QR, downloads PNG/SVG, and copies the link", async ({
    page,
  }) => {
    test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
    test.setTimeout(90_000)

    await signup(page)
    const eventName = `E2E QR ${Date.now()}`
    await page.goto("/events/new")
    await page.getByLabel("Event name").fill(eventName)
    await page.getByRole("button", { name: "Create event" }).click()
    await expect(page).toHaveURL(/\/events\/[0-9a-f-]+/, { timeout: 15_000 })

    await page.getByRole("button", { name: "QR code" }).click()
    await expect(page.getByText("Event QR code")).toBeVisible()
    await expect(
      page.getByRole("img", { name: `QR code for ${eventName}` })
    ).toBeVisible({ timeout: 15_000 })

    const pngDownload = page.waitForEvent("download")
    await page.getByRole("button", { name: /Download PNG/ }).click()
    expect((await pngDownload).suggestedFilename()).toMatch(/-qr\.png$/)

    const svgDownload = page.waitForEvent("download")
    await page.getByRole("button", { name: /Download SVG/ }).click()
    expect((await svgDownload).suggestedFilename()).toMatch(/-qr\.svg$/)

    await page.getByRole("button", { name: /Copy link/ }).click()
    await expect(page.getByText("Gallery link copied")).toBeVisible({
      timeout: 15_000,
    })
  })
})
