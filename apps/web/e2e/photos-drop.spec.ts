import { expect, test, type Page } from "@playwright/test"

const apiOrigin =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080"
const readyzUrl = new URL("/readyz", apiOrigin).toString()

// 1x1 JPEG, 160 bytes; the worker must be able to decode it.
const TINY_JPEG_BASE64 =
  "/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/wAALCAABAAEBAREA/8QAFAABAAAAAAAAAAAAAAAAAAAACf/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AKp//2Q=="

let apiAvailable = false

test.beforeAll(async ({ request }) => {
  try {
    const response = await request.get(readyzUrl, { timeout: 3000 })
    apiAvailable = response.ok()
  } catch {
    apiAvailable = false
  }
})

async function openPhotosTab(page: Page) {
  const email = `e2e-photo-drop-${Date.now()}@example.com`

  await page.goto("/signup")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password", { exact: true }).fill("password123")
  await page.getByRole("button", { name: "Create account" }).click()
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 })

  await page.goto("/events/new")
  await page.getByLabel("Event name").fill(`E2E Photo Drop ${Date.now()}`)
  await page.getByRole("button", { name: "Create event" }).click()
  await expect(page).toHaveURL(/\/events\/[0-9a-f-]+/, { timeout: 15_000 })

  await page.getByRole("tab", { name: "Photos" }).click()
  await expect(page.getByTestId("upload-dropzone")).toBeVisible()
}

test("dropping a JPEG on the dropzone queues it", async ({ page }) => {
  test.skip(!apiAvailable, `Go API not reachable at ${apiOrigin}`)
  test.setTimeout(90_000)

  await openPhotosTab(page)

  const fileName = `e2e-dropped-${Date.now()}.jpg`
  await page.evaluate(
    ({ name, base64 }) => {
      const binary = atob(base64)
      const bytes = new Uint8Array(binary.length)
      for (let i = 0; i < binary.length; i += 1) {
        bytes[i] = binary.charCodeAt(i)
      }
      const transfer = new DataTransfer()
      transfer.items.add(new File([bytes], name, { type: "image/jpeg" }))

      const zone = document.querySelector('[data-testid="upload-dropzone"]')
      if (!zone) throw new Error("dropzone not found")
      for (const type of ["dragenter", "dragover", "drop"]) {
        zone.dispatchEvent(
          new DragEvent(type, {
            bubbles: true,
            cancelable: true,
            dataTransfer: transfer,
          })
        )
      }
    },
    { name: fileName, base64: TINY_JPEG_BASE64 }
  )

  await expect(page.getByText(fileName).first()).toBeVisible({
    timeout: 10_000,
  })
})
