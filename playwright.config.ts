import { defineConfig } from '@playwright/test';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { randomUUID } from 'node:crypto';

export default defineConfig({
  testDir: './web/tests',
  workers: 1,
  use: { baseURL: 'http://127.0.0.1:18080', browserName: 'chromium' },
  webServer: {
    command: `${process.env.PIRATARR_TEST_EXECUTABLE ? JSON.stringify(process.env.PIRATARR_TEST_EXECUTABLE) : 'go run ./cmd/louro'} --listen 127.0.0.1:18080`,
    url: 'http://127.0.0.1:18080/api/v1/health/ready',
    timeout: 60_000,
    reuseExistingServer: false,
    env: { PIRATARR_DATA_DIR: join(tmpdir(), `piratarr-browser-${randomUUID()}`) },
  },
});
