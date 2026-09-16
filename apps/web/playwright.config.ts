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
  webServer: [
    {
      // The suite shares 127.0.0.1 and exhausts the API's per-IP auth rate
      // limit. The proxy injects a unique X-Forwarded-For per request; run the
      // Go API on :18081 for E2E (`HTTP_ADDR=:18081`).
      command: "node e2e/api-proxy.mjs",
      port: 18080,
      reuseExistingServer: true,
    },
    {
      command: "pnpm dev",
      url: "http://localhost:3000",
      reuseExistingServer: !process.env.CI,
    },
  ],
})
