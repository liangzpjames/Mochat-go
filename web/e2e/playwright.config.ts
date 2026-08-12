import { defineConfig, devices } from '@playwright/test';

const identitySingleCorpOnly = process.argv.some((argument) => argument.includes('identity-single-corp.spec.ts'));
const identitySingleCorpLiveConfigured = Boolean(
  process.env.MOCHAT_E2E_LIVE_BASE && process.env.MOCHAT_E2E_IDENTITY_SINGLE_CORP_FIXTURE_JSON,
);

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: 'http://127.0.0.1:4174',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  ...(identitySingleCorpOnly && !identitySingleCorpLiveConfigured
    ? {}
    : {
        webServer: {
          command: 'go run ./cmd/mochat-frontend-e2e',
          cwd: '../..',
          port: 4174,
          reuseExistingServer: !process.env.CI,
          timeout: 120_000,
        },
      }),
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        ...(process.env.MOCHAT_PLAYWRIGHT_CHANNEL
          ? { channel: process.env.MOCHAT_PLAYWRIGHT_CHANNEL }
          : {}),
      },
    },
  ],
});
