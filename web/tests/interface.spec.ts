import { expect, test, type BrowserContext } from '@playwright/test';

test.use({ locale: 'en-US', colorScheme: 'light' });

const setup = {
  completed: false,
  supportedResolutions: ['480p', '576p', '720p', '1080p', '2160p'],
  languages: [{ code: 'en', name: 'English' }, { code: 'pt', name: 'Portuguese' }],
};

async function unconfigured(context: BrowserContext) {
  await context.route('**/api/v1/setup', route => route.fulfill({ json: setup }));
}

test('theme and locale switch immediately without resetting setup, then persist across reload', async ({ page, context }) => {
  await unconfigured(context);
  let mutations = 0;
  page.on('request', request => { if (['POST', 'PUT'].includes(request.method())) mutations++; });
  await page.goto('/setup');
  await page.getByLabel('Download directory', { exact: true }).fill('/downloads');
  await page.getByLabel('Movie directory', { exact: true }).fill('/movies');
  await page.getByLabel('Language', { exact: true }).selectOption('pt-BR');
  await expect(page.locator('html')).toHaveAttribute('lang', 'pt-BR');
  await expect(page.getByLabel('Pasta de downloads', { exact: true })).toHaveValue('/downloads');
  await page.getByLabel('Tema', { exact: true }).selectOption('dark');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(16, 25, 24)');
  await expect(page.locator('html')).toHaveCSS('color-scheme', 'dark');
  await page.getByRole('button', { name: 'Continuar', exact: true }).click();
  await page.getByRole('button', { name: 'Continuar', exact: true }).click();
  await expect(page.getByText('Escolha pelo menos uma resolução aceita.')).toBeVisible();
  await page.getByLabel('1080p', { exact: true }).check();
  await page.getByLabel('Resolução preferida', { exact: true }).selectOption('1080p');
  await page.getByLabel('Idioma', { exact: true }).selectOption('en-US');
  await expect(page.getByRole('heading', { name: 'Choose your picture quality' })).toBeVisible();
  await expect(page.getByLabel('1080p', { exact: true })).toBeChecked();
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
  await page.getByRole('checkbox', { name: 'Portuguese', exact: true }).check();
  await page.getByRole('checkbox', { name: 'Original audio', exact: true }).check();
  await page.getByLabel('Language', { exact: true }).selectOption('pt-BR');
  await expect(page.getByRole('checkbox', { name: 'português', exact: true })).toBeChecked();
  await page.getByRole('button', { name: 'Mover Áudio original para cima', exact: true }).click();
  await page.getByLabel('Buscar idiomas de áudio').fill('portugues');
  await expect(page.getByRole('checkbox', { name: 'português', exact: true })).toBeVisible();
  await page.screenshot({ path: 'test-results/interface-dark-pt-BR.png', fullPage: true });
  await page.getByRole('button', { name: 'Continuar', exact: true }).click();
  await expect(page.getByText('Áudio original → português', { exact: true })).toBeVisible();
  await page.getByLabel('Tema', { exact: true }).selectOption('light');
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(241, 242, 233)');
  await expect(page.getByText('Áudio original → português', { exact: true })).toBeVisible();
  expect(mutations).toBe(0);
  await page.reload();
  await expect(page.getByLabel('Idioma', { exact: true })).toHaveValue('pt-BR');
  await expect(page.getByLabel('Tema', { exact: true })).toHaveValue('light');
  await expect(page.locator('html')).toHaveAttribute('lang', 'pt-BR');
  await page.getByRole('link', { name: 'Piratarr', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Deixe este espaço do seu jeito' })).toBeVisible();
  await page.setViewportSize({ width: 375, height: 812 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/interface-light-pt-BR-mobile.png', fullPage: true });
});

test('interface preferences do not mutate audio settings and conflict messages follow the current locale', async ({ page, context }) => {
  await context.route('**/api/v1/setup', route => route.fulfill({ json: { ...setup, completed: true } }));
  const settings = { revision: 4, directories: { downloadDirectory: '/downloads', movieDirectory: '/movies' }, profile: { acceptedResolutions: ['1080p'], preferredResolution: '1080p', audioPreferences: ['pt', 'original'] }, updatedAt: '2026-11-04T20:00:00Z' };
  let mutations = 0;
  await context.route('**/api/v1/settings', async route => {
    if (route.request().method() === 'PUT') {
      mutations++;
      const body = route.request().postDataJSON();
      expect(body.revision).toBe(4);
      expect(body.profile.audioPreferences).toEqual(['pt', 'original']);
      await route.fulfill({ status: 409, json: { code: 'revision-conflict', message: 'Settings changed in another tab. Reload the saved settings before editing again.' } });
    } else await route.fulfill({ json: settings });
  });
  await page.goto('/settings');
  await page.getByLabel('Language', { exact: true }).selectOption('pt-BR');
  await page.getByLabel('Tema', { exact: true }).selectOption('dark');
  await expect(page.getByText('português → Áudio original', { exact: true })).toBeVisible();
  expect(mutations).toBe(0);
  await page.getByRole('button', { name: 'Editar configurações', exact: true }).click();
  for (let index = 0; index < 3; index++) await page.getByRole('button', { name: 'Continuar', exact: true }).click();
  await page.getByRole('button', { name: 'Salvar alterações', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('As configurações foram alteradas em outra aba.');
  await expect(page.getByRole('button', { name: 'Salvar alterações', exact: true })).toBeDisabled();
  await page.getByLabel('Idioma', { exact: true }).selectOption('en-US');
  await expect(page.getByRole('alert')).toContainText('Settings changed in another tab.');
  expect(mutations).toBe(1);
});

test('invalid stored preferences fall back to Portuguese and system dark mode', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'pt-BR', colorScheme: 'dark' });
  try {
    await unconfigured(context);
    await context.addInitScript(() => {
      localStorage.setItem('piratarr.interface.theme', 'invalid');
      localStorage.setItem('piratarr.interface.locale', 'invalid');
    });
    const page = await context.newPage();
    await page.goto('/');
    await expect(page.locator('html')).toHaveAttribute('lang', 'pt-BR');
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await expect(page.getByRole('link', { name: 'Iniciar configuração' })).toBeVisible();
  } finally { await context.close(); }
});

test('blocked browser storage and backend failures remain usable and localized', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'pt-BR', colorScheme: 'light' });
  try {
    await context.addInitScript(() => { Object.defineProperty(window, 'localStorage', { get() { throw new DOMException('Blocked', 'SecurityError'); } }); });
    await context.route('**/api/v1/setup', route => route.fulfill({ status: 503, json: { code: 'storage-unavailable', message: 'Local settings are unavailable.' } }));
    const page = await context.newPage();
    await page.goto('/');
    await expect(page.getByRole('alert')).toHaveText('As configurações locais estão indisponíveis.');
    await page.getByLabel('Tema', { exact: true }).selectOption('dark');
    await page.getByLabel('Idioma', { exact: true }).selectOption('en-US');
    await expect(page.getByRole('alert')).toHaveText('Local settings are unavailable.');
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await expect(page.getByRole('button', { name: 'Try again' })).toBeVisible();
  } finally { await context.close(); }
});
