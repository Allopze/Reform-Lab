import { defineConfig, devices } from "@playwright/test";

const playwrightPort = process.env.PLAYWRIGHT_PORT ?? "5070";
const baseURL = `http://127.0.0.1:${playwrightPort}`;
const webServerCommand = `sh -ac 'set -a; . ../../.env; set +a; exec next dev -p ${playwrightPort}'`;

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL,
    trace: "on-first-retry",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: {
    command: webServerCommand,
    url: baseURL,
    timeout: 30_000,
    reuseExistingServer: !process.env.CI,
  },
});
