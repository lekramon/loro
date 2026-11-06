import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.route('**/api/v1/acquisitions?**', route => route.fulfill({ json: { items: [], total: 0, offset: 0, limit: 25 } }));
});

const movie = { id: 7, tmdbId: 42, title: 'The Example', year: 2024 };
const source = { configured: true, name: 'Local source', capabilities: { categories: [{ id: 2000, name: 'Movies' }] } };
const checks = [
  { dimension: 'resolution', status: 'match', accepted: ['1080p', '720p'] },
  { dimension: 'format', status: 'match', accepted: ['remux', 'web-dl'] },
  { dimension: 'container', status: 'unknown', accepted: ['mkv', 'mp4'] },
  { dimension: 'audio', status: 'unknown', accepted: ['original', 'pt'] },
];
const candidate = { id: 'a'.repeat(64), title: 'The.Example.2024.1080p.Remux', sizeBytes: 1234000000, seeders: 12, publishedAt: null, resolution: '1080p', format: 'remux', container: null, audioLanguages: [], status: 'unknown', preferredResolution: true, audioPreferenceRank: null, checks };

test('slow search can be cancelled and a challenge has a distinct localized error', async ({ page }) => {
  let searching = false; let release!: () => void;
  const hold = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/api/v1/catalog/movies?**', route => route.fulfill({ json: { items: [movie], total: 1, offset: 0, limit: 25 } }));
  await page.route('**/api/v1/source', route => route.fulfill({ json: source }));
  await page.route('**/api/v1/releases?**', async route => {
    if (!searching) { searching = true; await hold; }
    await route.fulfill({ status: 502, json: { code: 'source-challenge-required', message: 'Sanitized' } }).catch(() => {});
  });
  await page.goto('/preview#/releases');
  await page.getByRole('button', { name: 'The Example 2024', exact: true }).click();
  await page.getByRole('button', { name: 'Search releases', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Cancel search', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Cancel search', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Search releases', exact: true })).toBeEnabled();
  release();
  await page.getByRole('button', { name: 'Português (Brasil)', exact: true }).click();
  await page.getByRole('button', { name: 'Buscar lançamentos', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('A fonte exige verificação pelo navegador.');
});

test('TMDB registration leads to release search with profile explanations and localized layout', async ({ page }) => {
  let registrations = 0; const searches: URL[] = [];
  await page.route('**/api/v1/metadata/search?**', route => route.fulfill({ json: { page: 1, totalPages: 1, totalResults: 1, cache: { hit: false, stale: false }, results: [{ ...movie, overview: 'A simulated movie.', posterUrl: '' }] } }));
  await page.route('**/api/v1/catalog/movies', route => { registrations++; expect(route.request().postDataJSON()).toEqual({ tmdbId: 42, language: 'en-US' }); return route.fulfill({ status: 201, json: { movie, created: true } }); });
  await page.route('**/api/v1/catalog/movies?**', route => route.fulfill({ json: { items: [movie], total: 1, offset: 0, limit: 25 } }));
  await page.route('**/api/v1/catalog/movies/7', route => route.fulfill({ json: movie }));
  await page.route('**/api/v1/source', route => route.fulfill({ json: source }));
  await page.route('**/api/v1/releases?**', route => { searches.push(new URL(route.request().url())); return route.fulfill({ json: { items: [candidate, { ...candidate, id: 'b'.repeat(64), title: 'The.Example.2160p.WEB-DL.mp4', resolution: '2160p', format: 'web-dl', container: 'mp4', status: 'mismatch', preferredResolution: false, checks: checks.map(item => item.dimension === 'resolution' ? { ...item, status: 'mismatch' } : item.dimension === 'container' ? { ...item, status: 'match' } : item) }], sourceName: 'Local source', sourceRevision: 1, profileRevision: 2, limit: 25, offset: 0, total: 2, nextOffset: null, hasMore: false } }); });
  await page.goto('/preview#/search'); await page.getByLabel('Movie title', { exact: true }).fill('Example'); await page.getByRole('button', { name: 'Search', exact: true }).click();
  await page.getByRole('button', { name: 'Find releases', exact: true }).click(); await expect(page).toHaveURL(/#\/releases\?movieId=7$/); expect(registrations).toBe(1);
  await expect(page.getByRole('heading', { name: 'The Example 2024' })).toBeVisible(); expect(searches).toHaveLength(0);
  await page.getByLabel('Category', { exact: true }).selectOption('2000'); await page.getByRole('button', { name: 'Search releases', exact: true }).click();
  await expect(page.locator('.release-card')).toHaveCount(2); await expect(page.locator('.release-card').first()).toContainText('Unknown'); await expect(page.locator('.release-card').nth(1)).toContainText('Outside profile');
  expect(searches[0].searchParams.get('movieId')).toBe('7'); expect(searches[0].searchParams.get('category')).toBe('2000'); await expect(page.getByRole('button', { name: 'Next releases', exact: true })).toBeDisabled();
  await expect(page.locator('.release-results').getByRole('button', { name: /download/i })).toHaveCount(0); await expect(page.locator('.persistent-sidebar')).toBeVisible();
  await page.getByRole('button', { name: 'Dark', exact: true }).click(); await page.screenshot({ path: 'test-results/release-search-dark.png', fullPage: true });
  await page.getByRole('button', { name: 'Português (Brasil)', exact: true }).click(); await expect(page.getByRole('heading', { name: 'Lançamentos', exact: true })).toBeVisible(); await expect(page.locator('.release-card').nth(1)).toContainText('Fora do perfil');
  await page.setViewportSize({ width: 390, height: 844 }); await page.getByRole('button', { name: 'Claro', exact: true }).click(); await page.evaluate(() => window.scrollTo(0, 0)); expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true); await page.screenshot({ path: 'test-results/release-search-mobile.png', fullPage: true });
});

test('registered movie search works without TMDB and uses raw-provider next offsets', async ({ page }) => {
  let metadataCalls = 0; let offset = -1; let reject = false;
  await page.route('**/api/v1/metadata/**', route => { metadataCalls++; return route.abort(); });
  await page.route('**/api/v1/catalog/movies?**', route => route.fulfill({ json: { items: [movie], total: 1, offset: 0, limit: 25 } }));
  await page.route('**/api/v1/source', route => route.fulfill({ json: source }));
  await page.route('**/api/v1/releases?**', route => {
    offset = Number(new URL(route.request().url()).searchParams.get('offset'));
    return reject ? route.fulfill({ status: 503, json: { code: 'source-authentication-failed', message: 'Sanitized' } }) : route.fulfill({ json: { items: offset ? [] : [candidate], sourceName: 'Local source', sourceRevision: 1, profileRevision: 1, limit: 25, offset, total: null, hasMore: !offset, nextOffset: offset ? null : 2 } });
  });
  await page.goto('/preview#/releases'); await page.getByRole('button', { name: 'The Example 2024', exact: true }).click(); await page.getByRole('button', { name: 'Search releases', exact: true }).click(); await expect(page.locator('.release-card')).toHaveCount(1);
  await page.getByRole('button', { name: 'Next releases', exact: true }).click(); await expect(page.getByText('No releases found.', { exact: true })).toBeVisible(); expect(offset).toBe(2); expect(metadataCalls).toBe(0);
  reject = true; await page.getByRole('button', { name: 'Search releases', exact: true }).click(); await expect(page.getByRole('alert')).toContainText('The source rejected its credentials.'); await expect(page.locator('.release-card')).toHaveCount(0); await expect(page.getByRole('alert').getByRole('link', { name: 'Settings', exact: true })).toBeVisible();
});
