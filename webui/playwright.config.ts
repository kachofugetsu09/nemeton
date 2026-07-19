import { defineConfig } from "@playwright/test"

const browser = process.env.NEMETON_PLAYWRIGHT_BUNDLED === "1" ? {} : { channel: "chrome" as const }

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  reporter: "line",
  use: {
    baseURL: "http://127.0.0.1:4173",
    ...browser,
    headless: true,
    trace: "retain-on-failure",
  },
  webServer: {
    command: "go run ../cmd/nemetond -data-dir /tmp/nemeton-playwright -web-address 127.0.0.1:4173",
    url: "http://127.0.0.1:4173",
    reuseExistingServer: !process.env.CI,
  },
})
