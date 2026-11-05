import { expect, test } from '@playwright/test';

test('metadata token is write-only, persists and supports replacement in settings', async ({ page }) => {
  await page.goto('/setup');
  await expect(page.getByLabel('TMDB API Read Access Token')).toHaveAttribute('type', 'password');
  const token = 'dummy-browser-token';
  await page.getByLabel('TMDB API Read Access Token').fill(token);
  await page.getByRole('button', { name: 'Save TMDB token', exact: true }).click();
  await expect(page.getByText('Token saved. Test the connection to verify access.')).toBeVisible();
  await expect(page.getByLabel('TMDB API Read Access Token')).toHaveValue('');
  await page.reload();
  await expect(page.getByText('TMDB token configured', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toContain(token);
  expect(await (await page.request.get('/api/v1/metadata/status')).text()).not.toContain(token);
  await expect(page.getByRole('button', { name: 'Save TMDB token', exact: true })).toBeDisabled();
  await page.getByLabel('TMDB API Read Access Token').fill('dummy-browser-replacement');
  await page.getByRole('button', { name: 'Save TMDB token', exact: true }).click();
  await expect(page.getByText('Token saved. Test the connection to verify access.')).toBeVisible();
  await page.getByRole('button', {name:'Dark',exact:true}).click();
  await page.evaluate(() => window.scrollTo(0,0));
  await page.screenshot({path:'test-results/service-settings-dark.png',fullPage:true});
  await page.setViewportSize({width:390,height:844});
  await page.getByRole('button', {name:'Light',exact:true}).click();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.evaluate(() => window.scrollTo(0,0));
  await page.screenshot({path:'test-results/service-settings-mobile-light.png',fullPage:true});
});


test('connection test reports real request success and localized failure, with saved token only', async ({ page }) => {
  await page.goto('/settings');
  await page.route('**/api/v1/metadata/test', route => route.fulfill({ json: {provider:'tmdb',connected:true} }));
  await page.getByRole('button',{name:'Test connection',exact:true}).click();
  await expect(page.getByText('TMDB connection successful.',{exact:true})).toBeVisible();
  await page.getByLabel('TMDB API Read Access Token').fill('unsaved');
  await expect(page.getByRole('button',{name:'Test connection',exact:true})).toBeDisabled();
  await page.getByLabel('TMDB API Read Access Token').fill('');
  await page.getByLabel('Language',{exact:true}).selectOption('pt-BR');
  await page.route('**/api/v1/metadata/test', route => route.fulfill({status:503,json:{code:'metadata-authentication-failed',message:'Sanitized'}}));
  await page.getByRole('button',{name:'Testar conexão',exact:true}).click();
  await expect(page.getByText(/O TMDB rejeitou o token/)).toBeVisible();
});
