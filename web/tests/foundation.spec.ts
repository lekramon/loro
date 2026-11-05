import { expect, test, type Page } from '@playwright/test';
import { mkdtemp, mkdir, readdir, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

test.describe.configure({ mode: 'serial' });

test('unconfigured installation explains its state and opens setup', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByText('Not configured yet', { exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Make this space yours' })).toBeVisible();
  await page.getByRole('link', { name: 'Start setup' }).click();
  await expect(page).toHaveURL(/\/setup$/);
  await expect(page.getByRole('heading', { name: 'Choose your local folders' })).toBeVisible();
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
  await expect(page.getByText('Enter your existing download directory.')).toBeVisible();
  await page.reload();
  await expect(page.getByLabel('Download directory', { exact: true })).toHaveValue('');
});

async function fillWizard(page: Page, downloads: string, movies: string) {
  await page.goto('/setup');
  await page.getByLabel('Download directory', { exact: true }).fill(downloads);
  await page.getByLabel('Movie directory', { exact: true }).fill(movies);
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
  await expect(page.getByLabel('1080p', { exact: true })).not.toBeChecked();
  await expect(page.getByLabel('Preferred resolution', { exact: true })).toHaveValue('');
  await page.getByLabel('720p', { exact: true }).check();
  await page.getByLabel('1080p', { exact: true }).check();
  await page.getByLabel('Preferred resolution', { exact: true }).selectOption('1080p');
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
  await expect(page.getByRole('checkbox', { name: 'Original audio', exact: true })).not.toBeChecked();
  await page.getByLabel('Search audio languages').fill('Portuguese');
  await page.getByRole('checkbox', { name: 'Portuguese', exact: true }).check();
  await page.getByLabel('Search audio languages').fill('Original');
  await page.getByRole('checkbox', { name: 'Original audio', exact: true }).check();
  await page.getByRole('button', { name: 'Move Original audio up', exact: true }).click();
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Review your setup' })).toBeVisible();
  await expect(page.getByText('Original audio → Portuguese', { exact: true })).toBeVisible();
}

test('onboarding validates paths, saves ranked choices, and handles another tab completing setup', async ({ page, browser, request }) => {
  const root = await mkdtemp(join(tmpdir(), 'piratarr-setup-browser-'));
  const downloads = join(root, 'downloads');
  const movies = join(root, 'movies');
  await mkdir(downloads); await mkdir(movies);
  await writeFile(join(movies, 'existing.mkv'), 'existing media');
  const other = await browser.newContext();
  try {
    const secondPage = await other.newPage();
    await fillWizard(page, join(root, 'missing'), movies);
    await fillWizard(secondPage, downloads, movies);
    await page.getByRole('button', { name: 'Save setup', exact: true }).click();
    await expect(page.getByText('Directory does not exist or is inaccessible.')).toBeVisible();
    await page.getByLabel('Download directory', { exact: true }).fill(downloads);
    for (let index = 0; index < 3; index++) await page.getByRole('button', { name: 'Continue', exact: true }).click();
    await page.getByRole('button', { name: 'Save setup', exact: true }).click();
    await expect(page).toHaveURL(/\/settings$/);
    await expect(page.getByRole('heading', { name: 'Your saved settings' })).toBeVisible();
    await expect(page.getByText('Original audio → Portuguese', { exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByText('Configured · revision 1', { exact: true })).toBeVisible();
    const settings = await (await request.get('/api/v1/settings')).json();
    expect(settings.profile.acceptedResolutions).toEqual(['720p', '1080p']);
    expect(settings.profile.audioPreferences).toEqual(['original', 'pt']);
    await secondPage.getByRole('button', { name: 'Save setup', exact: true }).click();
    await expect(secondPage.getByRole('alert')).toContainText('Setup has already been completed');
    await secondPage.getByRole('button', { name: 'Reload saved settings' }).click();
    await expect(secondPage.getByRole('heading', { name: 'Your saved settings' })).toBeVisible();
    expect(await readFile(join(movies, 'existing.mkv'), 'utf8')).toBe('existing media');
    expect(await readdir(downloads)).toEqual([]);
    expect(await readdir(movies)).toEqual(['existing.mkv']);
    await page.screenshot({ path: 'test-results/setup-saved.png', fullPage: true });
  } finally {
    await other.close();
    // Delete only the temporary tree created by this test.
    if (resolve(root).startsWith(resolve(tmpdir()) + '\\piratarr-setup-browser-') || resolve(root).startsWith(resolve(tmpdir()) + '/piratarr-setup-browser-')) await rm(root, { recursive: true, force: true });
  }
});

test('unknown API resources remain JSON instead of returning the application shell', async ({ request }) => {
  const response = await request.get('/api/v1/missing');
  expect(response.status()).toBe(404);
  expect(response.headers()['content-type']).toContain('application/json');
  expect((await response.json()).code).toBe('not-found');
});
