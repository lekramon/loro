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
  await expect(page.getByRole('heading', {level:1})).toHaveText('Getting started');
  await expect(page.locator('.wizard-progress [aria-current=step]')).toHaveText('1Folders');
  await expect(page.locator('.workspace-nav [aria-current=page]')).toContainText('Setup');
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
  await expect(page.getByLabel('1080p', { exact: true })).toBeChecked();
  for (const resolution of ['480p', '576p', '2160p']) { await expect(page.getByLabel(resolution, {exact:true})).toBeChecked(); await page.getByLabel(resolution, {exact:true}).uncheck(); }
  await expect(page.getByLabel('REMUX', {exact:true})).toBeChecked();
  await page.getByLabel('REMUX', {exact:true}).uncheck();
  await expect(page.getByLabel('Preferred resolution', { exact: true })).toHaveValue('');
  await page.getByLabel('720p', { exact: true }).check();
  await page.getByLabel('1080p', { exact: true }).check();
  await page.getByLabel('Preferred resolution', { exact: true }).selectOption('1080p');
  await page.evaluate(()=>window.scrollTo(0,0)); await page.screenshot({path:'test-results/cinematic-quality-desktop.png',fullPage:true});
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

test('onboarding and editing preserve preferences and require reload for conflicting tabs', async ({ page, browser, request }) => {
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
    expect(settings.profile.acceptedFormats).not.toContain('remux');
    expect(settings.profile.acceptedContainers).toContain('mkv');
    await secondPage.getByRole('button', { name: 'Save setup', exact: true }).click();
    await expect(secondPage.getByRole('alert')).toContainText('Setup has already been completed');
    await secondPage.getByRole('button', { name: 'Reload saved settings' }).click();
    await expect(secondPage.getByRole('heading', { name: 'Your saved settings' })).toBeVisible();

    // Both editors start from revision 1. Failed validation must keep the
    // server revision intact, while successful saving must stale the other tab.
    const alternateMovies = join(root, 'alternate-movies');
    await mkdir(alternateMovies);
    await page.getByRole('button', { name: 'Edit settings', exact: true }).click();
    await expect(page.getByLabel('Download directory', { exact: true })).toHaveValue(settings.directories.downloadDirectory);
    await expect(page.getByLabel('Movie directory', { exact: true })).toHaveValue(settings.directories.movieDirectory);
    await page.getByLabel('Movie directory', { exact: true }).fill(join(root, 'missing-edit'));
    await page.getByRole('button', { name: 'Continue', exact: true }).click();
    await expect(page.getByLabel('1080p', { exact: true })).toBeChecked();
    await page.getByLabel('2160p', { exact: true }).check();
    await page.getByLabel('Preferred resolution', { exact: true }).selectOption('2160p');
    await page.getByRole('button', { name: 'Continue', exact: true }).click();
    await page.getByRole('button', { name: 'Move Portuguese up', exact: true }).click();
    await page.getByLabel('Search audio languages').fill('English');
    await page.getByRole('checkbox', { name: 'English', exact: true }).check();
    await page.getByRole('button', { name: 'Continue', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Review your changes' })).toBeVisible();
    await page.getByRole('button', { name: 'Save changes', exact: true }).click();
    await expect(page.getByText('Directory does not exist or is inaccessible.')).toBeVisible();
    expect((await (await request.get('/api/v1/settings')).json()).revision).toBe(1);
    await page.getByLabel('Movie directory', { exact: true }).fill(alternateMovies);
    for (let index = 0; index < 3; index++) await page.getByRole('button', { name: 'Continue', exact: true }).click();

    await secondPage.getByRole('button', { name: 'Edit settings', exact: true }).click();
    for (let index = 0; index < 3; index++) await secondPage.getByRole('button', { name: 'Continue', exact: true }).click();
    await page.getByRole('button', { name: 'Save changes', exact: true }).click();
    await expect(page.getByText('Configured · revision 2', { exact: true })).toBeVisible();
    await expect(page.getByText('Portuguese → Original audio → English', { exact: true })).toBeVisible();
    await secondPage.getByRole('button', { name: 'Save changes', exact: true }).click();
    await expect(secondPage.getByRole('alert')).toContainText('Settings changed in another tab');
    await expect(secondPage.getByRole('button', { name: 'Save changes', exact: true })).toBeDisabled();
    expect((await (await request.get('/api/v1/settings')).json()).revision).toBe(2);
    await secondPage.getByRole('button', { name: 'Reload saved settings' }).click();
    await expect(secondPage.getByText('Configured · revision 2', { exact: true })).toBeVisible();
    await expect(secondPage.getByText('Portuguese → Original audio → English', { exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByText('Configured · revision 2', { exact: true })).toBeVisible();
    const updated = await (await request.get('/api/v1/settings')).json();
    expect(updated.profile.preferredResolution).toBe('2160p');
    expect(updated.profile.audioPreferences).toEqual(['pt', 'original', 'en']);
    await page.getByRole('button', { name: 'Edit settings', exact: true }).click();
    await page.getByLabel('Movie directory', { exact: true }).fill(movies);
    await page.getByRole('button', { name: 'Cancel editing', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Your saved settings' })).toBeVisible();
    expect((await (await request.get('/api/v1/settings')).json()).directories.movieDirectory).toBe(updated.directories.movieDirectory);
    expect(await readFile(join(movies, 'existing.mkv'), 'utf8')).toBe('existing media');
    expect(await readdir(downloads)).toEqual([]);
    expect(await readdir(movies)).toEqual(['existing.mkv']);
    expect(await readdir(alternateMovies)).toEqual([]);
    await page.evaluate(()=>window.scrollTo(0,0)); await page.screenshot({ path: 'test-results/settings-updated.png', fullPage: true });
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
