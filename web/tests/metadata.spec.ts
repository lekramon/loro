import { expect, test } from '@playwright/test';

test('metadata token is write-only, persists and supports replacement in settings', async ({ page }) => {
  await page.goto('/setup');
  await expect(page.getByLabel('TMDB API Read Access Token')).toHaveAttribute('type', 'password');
  const token = 'dummy-browser-token';
  await page.getByLabel('TMDB API Read Access Token').fill(token);
  await page.getByRole('button', { name: 'Save TMDB token', exact: true }).click();
  await expect(page.getByText('Token saved. Movie search can now use it.')).toBeVisible();
  await expect(page.getByLabel('TMDB API Read Access Token')).toHaveValue('');
  await page.reload();
  await expect(page.getByText('TMDB token configured', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toContain(token);
  expect(await (await page.request.get('/api/v1/metadata/status')).text()).not.toContain(token);
  await expect(page.getByRole('button', { name: 'Save TMDB token', exact: true })).toBeDisabled();
  await page.getByLabel('TMDB API Read Access Token').fill('dummy-browser-replacement');
  await page.getByRole('button', { name: 'Save TMDB token', exact: true }).click();
  await expect(page.getByText('Token saved. Movie search can now use it.')).toBeVisible();
  await page.getByRole('button', {name:'Dark',exact:true}).click();
  await page.evaluate(() => window.scrollTo(0,0));
  await page.screenshot({path:'test-results/service-settings-dark.png',fullPage:true});
  await page.setViewportSize({width:390,height:844});
  await page.getByRole('button', {name:'Light',exact:true}).click();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.evaluate(() => window.scrollTo(0,0));
  await page.screenshot({path:'test-results/service-settings-mobile-light.png',fullPage:true});
});
