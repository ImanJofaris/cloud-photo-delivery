import { defineConfig, devices } from "@playwright/test"
import nextEnv from "@next/env"

nextEnv.loadEnvConfig(process.cwd())

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  // The Go API rate-limits auth endpoints per IP (30/min, burst 10). Parallel
  // workers share localhost and exhaust the bucket, so run one at a time.
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "html",
  use: {
    baseURL: "http://localhost:3000",
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: "pnpm dev",
    url: "http://localhost:3000",
    reuseExistingServer: !process.env.CI,
  },
})
