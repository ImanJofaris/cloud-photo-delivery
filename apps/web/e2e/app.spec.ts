import { expect, test } from "@playwright/test"

test("renders the app shell", async ({ page }) => {
  await page.goto("/")

  await expect(
    page.getByRole("heading", { name: "Cloud Photo Delivery" })
  ).toBeVisible()
  await expect(page.getByTestId("api-status")).toBeVisible()
  await expect(page.getByRole("button", { name: "Get started" })).toBeVisible()
})
