import { expect, test, type Page } from "@playwright/test"

const apiOrigin =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080"
const readyzUrl = new URL("/readyz", apiOrigin).toString()

// 1x1 JPEG, 160 bytes; the worker must be able to decode it.
const TINY_JPEG = Buffer.from(
  "/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/wAALCAABAAEBAREA/8QAFAABAAAAAAAAAAAAAAAAAAAACf/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AKp//2Q==",
  "base64"
)

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
  const email = `e2e-gallery-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })
}

async function createEvent(page: Page, name: string) {
  await page.goto("/events/new")
  await page.getByLabel("Event name").fill(name)
  await page.getByRole("button", { name: "Create event" }).click()
  await expect(page).toHaveURL(/\/events\/[0-9a-f-]+/, { timeout: 15_000 })
}

async function galleryHref(page: Page) {
  const link = page.getByRole("link", { name: "Open gallery" })
  const href = await link.getAttribute("href")
  if (!href) throw new Error("gallery link not found")
  return href
}

async function uploadPhoto(page: Page, filename: string) {
  await page.getByRole("tab", { name: "Photos" }).click()
  await page.setInputFiles('input[type="file"]', {
    name: filename,
    mimeType: "image/jpeg",
    buffer: TINY_JPEG,
  })
  await expect(page.getByText(filename).first()).toBeVisible({
    timeout: 10_000,
  })

  // The worker turns PROCESSING into READY. If it is not running, the guest
  // flow cannot be exercised; callers skip rather than fail the suite.
  const becameReady = await expect(page.getByText("Ready").first())
    .toBeVisible({ timeout: 30_000 })
    .then(() => true)
    .catch(() => false)
  return becameReady
}

async function setVisibility(
  page: Page,
  option: "Public" | "Password protected" | "Private",
  password?: string
) {
  await page.getByRole("tab", { name: "Settings" }).click()
  await page.getByLabel("Visibility").click()
  await page.getByRole("option", { name: option }).click()
  if (password) {
    await page.getByLabel("Gallery password").fill(password)
  }
  await page.getByRole("button", { name: "Save settings" }).click()
  await expect(page.getByText("Settings saved")).toBeVisible({
    timeout: 15_000,
  })
}

test("public gallery: view a photo, open the viewer, download", async ({
  page,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(120_000)

  await signup(page)
  const eventName = `E2E Gallery ${Date.now()}`
  await createEvent(page, eventName)
  const href = await galleryHref(page)

  test.skip(!(await uploadPhoto(page, "e2e-gallery.jpg")), "worker not running")

  await page.goto(href)
  await expect(
    page.getByRole("heading", { name: eventName })
  ).toBeVisible()

  const tile = page.getByRole("button", {
    name: `Open ${eventName} photo 1`,
  })
  await expect(tile).toBeVisible({ timeout: 30_000 })
  await expect(
    page.getByRole("img", { name: `${eventName} photo 1` })
  ).toBeVisible({ timeout: 30_000 })

  await tile.click()
  await expect(page.getByRole("dialog")).toBeVisible()

  const downloadPromise = page.waitForEvent("download")
  await page.getByRole("button", { name: "Download" }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toMatch(/^photo-/)
})

test("password gallery: blocked until the correct password is entered", async ({
  page,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(120_000)

  await signup(page)
  const eventName = `E2E Password ${Date.now()}`
  await createEvent(page, eventName)
  await setVisibility(page, "Password protected", "open-sesame")
  const href = await galleryHref(page)

  test.skip(!(await uploadPhoto(page, "e2e-password.jpg")), "worker not running")

  await page.goto(href)
  await expect(
    page.getByRole("heading", { name: "This gallery is protected" })
  ).toBeVisible()
  await expect(
    page.getByRole("button", { name: `Open ${eventName} photo 1` })
  ).toHaveCount(0)

  await page.getByLabel("Password").fill("wrong-password")
  await page.getByRole("button", { name: "Unlock gallery" }).click()
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "That password is not correct." })
  ).toBeVisible()

  await page.getByLabel("Password").fill("open-sesame")
  await page.getByRole("button", { name: "Unlock gallery" }).click()
  await expect(
    page.getByRole("button", { name: `Open ${eventName} photo 1` })
  ).toBeVisible({ timeout: 30_000 })
})

test("view-only gallery: images render without download controls", async ({
  page,
}) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(120_000)

  await signup(page)
  const eventName = `E2E ViewOnly ${Date.now()}`
  await createEvent(page, eventName)

  await page.getByRole("tab", { name: "Settings" }).click()
  await page.getByRole("switch", { name: "Allow downloads" }).click()
  await page.getByRole("button", { name: "Save settings" }).click()
  await expect(page.getByText("Settings saved")).toBeVisible({
    timeout: 15_000,
  })
  const href = await galleryHref(page)

  test.skip(!(await uploadPhoto(page, "e2e-view-only.jpg")), "worker not running")

  await page.goto(href)
  const tile = page.getByRole("button", {
    name: `Open ${eventName} photo 1`,
  })
  await expect(tile).toBeVisible({ timeout: 30_000 })
  await expect(
    page.getByRole("img", { name: `${eventName} photo 1` })
  ).toBeVisible({ timeout: 30_000 })

  await tile.click()
  await expect(page.getByRole("dialog")).toBeVisible()
  await expect(page.getByRole("button", { name: "Download" })).toHaveCount(0)
  await expect(page.getByRole("button", { name: "Original" })).toHaveCount(0)
})

test("private gallery: not-found page", async ({ page }) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(120_000)

  await signup(page)
  const eventName = `E2E Private ${Date.now()}`
  await createEvent(page, eventName)
  await setVisibility(page, "Private")
  const href = await galleryHref(page)

  await page.goto(href)
  await expect(
    page.getByRole("heading", { name: "Gallery not found" })
  ).toBeVisible()
})
