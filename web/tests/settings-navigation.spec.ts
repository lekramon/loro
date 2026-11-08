import { expect, test } from '@playwright/test';

const settings = { revision: 1, updatedAt: '2026-11-08T00:00:00Z', directories: { downloadDirectory: '/downloads', movieDirectory: '/movies' }, profile: { acceptedResolutions: ['1080p'], preferredResolution: '1080p', acceptedFormats: ['remux'], acceptedContainers: ['mkv'], audioPreferences: ['original'] } };

test('settings tree routes existing controls and keeps planned sections inert', async ({ page }) => {
  await page.route('**/api/v1/setup', route => route.fulfill({ json: { completed: true, supportedResolutions: ['1080p'], languages: [] } }));
  await page.route('**/api/v1/settings', route => route.fulfill({ json: settings }));
  await page.route('**/api/v1/sources', route => route.fulfill({ json: { activeSourceId: null, items: [] } }));
  const external: string[] = [];
  page.on('request', request => { if (request.url().includes('/api/v1/') && !request.url().endsWith('/setup') && !request.url().endsWith('/settings') && !request.url().endsWith('/sources')) external.push(request.url()); });

  await page.goto('/settings/quality');
  await expect(page.getByRole('heading', { name: 'Quality', exact: true })).toBeVisible();
  await expect(page.getByText('This section is planned and does not change local data yet.', { exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Indexers', exact: true })).toHaveAttribute('href', '/settings/indexers');
  expect(external).toEqual([]);

  await page.goto('/settings/indexers');
  await expect(page.getByRole('heading', { name: 'Release source', exact: true })).toBeVisible();
  await expect(page.getByRole('tab', { name: 'Add source', exact: true })).toBeVisible();

  await page.goto('/preview#/series');
  await expect(page.getByRole('heading', { name: 'Series', exact: true })).toBeVisible();
  await expect(page.getByText('Series and anime catalog support is planned.', { exact: false })).toBeVisible();
  expect(external).toEqual([]);
});
