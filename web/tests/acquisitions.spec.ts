import { expect, test } from '@playwright/test';

const movie = { id: 7, title: 'The Example', year: 2024 };
const candidate = { id: 'a'.repeat(64), observationId: '1'.repeat(32), title: 'The.Example.1080p.Remux', resolution: '1080p', format: 'remux', container: null, audioLanguages: [], sizeBytes: null, seeders: null, publishedAt: null, preferredResolution: true, audioPreferenceRank: null, status: 'unknown', checks: [
  { dimension: 'resolution', status: 'match', accepted: ['1080p'] }, { dimension: 'format', status: 'match', accepted: ['remux'] }, { dimension: 'container', status: 'unknown', accepted: ['mkv'] }, { dimension: 'audio', status: 'unknown', accepted: ['original'] },
] };
const saved = { id: 'b'.repeat(32), movieId: 7, movieTitle: movie.title, candidate, state: 'selected', revision: 1, infoHash: null as string | null, kind: null as string | null, reviewCode: null as string | null, overrideConfirmed: true, updatedAt: '2026-11-05T20:00:00Z' };

test('selection requires explicit override, reuses uncertain request key and reads durable history', async ({ page }) => {
  let record: typeof saved | undefined; const keys: string[] = []; let preparations = 0; let settingsMutations = 0;
  await page.route('**/api/v1/catalog/movies?**', route => route.fulfill({ json: { items: [movie], total: 1, offset: 0, limit: 25 } }));
  await page.route('**/api/v1/catalog/movies/7', route => route.fulfill({ json: movie }));
  await page.route('**/api/v1/source', route => route.fulfill({ json: { configured: true, name: 'Local source', capabilities: { categories: [] } } }));
  await page.route('**/api/v1/releases?**', route => route.fulfill({ json: { items: [candidate], sourceName: 'Local source', sourceRevision: 1, profileRevision: 1, limit: 25, offset: 0, total: 1, nextOffset: null, hasMore: false } }));
  await page.route('**/api/v1/acquisitions?**', route => route.fulfill({ json: { items: record ? [record] : [], total: record ? 1 : 0, offset: 0, limit: 25 } }));
  await page.route('**/api/v1/acquisitions', route => {
    const body = route.request().postDataJSON(); expect(body.observationId).toBe(candidate.observationId); expect(body.confirmOverride).toBe(true); expect(route.request().headers()['x-piratarr-request']).toBe('1'); keys.push(body.idempotencyKey);
    record ??= { ...saved }; if (keys.length === 1) return route.abort('failed');
    return route.fulfill({ json: { acquisition: record, created: false } });
  });
  await page.route('**/api/v1/acquisitions/' + saved.id + '/prepare', route => { preparations++; expect(route.request().postDataJSON()).toEqual({ revision: 1 }); record = { ...record!, state: 'ready', revision: 3, infoHash: '0123456789abcdef0123456789abcdef01234567', kind: 'magnet' }; return route.fulfill({ json: record }); });
  await page.route('**/api/v1/acquisitions/' + saved.id + '/cancel', route => { expect(route.request().postDataJSON()).toEqual({ revision: 3 }); record = { ...record!, state: 'cancelled', revision: 4 }; return route.fulfill({ json: record }); });
  page.on('request', request => { if (request.url().endsWith('/api/v1/settings') && request.method() === 'PUT') settingsMutations++; });
  await page.goto('/preview#/releases?movieId=7'); await page.getByRole('button', { name: 'Search releases', exact: true }).click(); await page.getByRole('button', { name: 'Select release', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Select release', exact: true }); await expect(dialog.getByRole('button', { name: 'Prepare acquisition', exact: true })).toBeDisabled(); await dialog.getByRole('button', { name: 'Cancel', exact: true }).click(); expect(keys).toHaveLength(0);
  await page.getByRole('button', { name: 'Select release', exact: true }).click(); await dialog.getByRole('checkbox').check(); await page.screenshot({ path: 'test-results/acquisition-override-dialog.png' }); await dialog.getByRole('button', { name: 'Prepare acquisition', exact: true }).click(); await expect(dialog.getByRole('alert')).toContainText('Refresh history');
  await dialog.getByRole('button', { name: 'Prepare acquisition', exact: true }).click(); await expect(dialog).not.toBeVisible(); await expect(page.getByText('Torrent identity ready', { exact: true })).toBeVisible(); expect(keys).toHaveLength(2); expect(keys[0]).toBe(keys[1]); expect(preparations).toBe(1); expect(settingsMutations).toBe(0);
  await page.getByRole('button', { name: 'Dark', exact: true }).click(); await page.evaluate(() => window.scrollTo(0, 0)); await page.screenshot({ path: 'test-results/acquisition-history-dark.png', fullPage: true });
  await page.reload(); await expect(page.getByText('Torrent identity ready', { exact: true })).toBeVisible(); await page.getByRole('button', { name: 'Português (Brasil)', exact: true }).click(); await expect(page.getByText('Identidade do torrent pronta', { exact: true })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 }); await page.getByRole('button', { name: 'Claro', exact: true }).click(); await page.evaluate(() => window.scrollTo(0, 0)); expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true); await page.screenshot({ path: 'test-results/acquisition-history-mobile.png', fullPage: true });
  await page.getByRole('button', { name: 'Cancelar aquisição', exact: true }).click(); await expect(page.getByText('Cancelada', { exact: true })).toBeVisible(); expect(preparations).toBe(1);
});

test('history keeps review reasons and refreshes stale revisions before explicit retry', async ({ page }) => {
  let record = { ...saved, state: 'needs-review', reviewCode: 'source-timeout' as string | null }; let attempts = 0;
  await page.route('**/api/v1/catalog/movies?**', route => route.fulfill({ json: { items: [movie], total: 1, offset: 0, limit: 25 } })); await page.route('**/api/v1/catalog/movies/7', route => route.fulfill({ json: movie })); await page.route('**/api/v1/source', route => route.fulfill({ json: { configured: true, name: 'Source', capabilities: { categories: [] } } }));
  await page.route('**/api/v1/acquisitions?**', route => route.fulfill({ json: { items: [record], total: 1, offset: 0, limit: 25 } }));
  await page.route('**/api/v1/acquisitions/' + saved.id + '/prepare', route => { attempts++; if (attempts === 1) { record = { ...record, revision: 2 }; return route.fulfill({ status: 409, json: { code: 'acquisition-changed', message: 'Sanitized' } }); } expect(route.request().postDataJSON()).toEqual({ revision: 2 }); record = { ...record, state: 'ready', revision: 4, infoHash: '0123456789abcdef0123456789abcdef01234567', kind: 'torrent', reviewCode: null }; return route.fulfill({ json: record }); });
  await page.goto('/preview#/releases?movieId=7'); await expect(page.getByText('Needs review', { exact: true })).toBeVisible(); await expect(page.getByText('The source timed out. Check its address and connectivity.', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Retry identity preparation', exact: true }).click(); await expect(page.getByRole('alert')).toContainText('Acquisition changed'); await page.getByRole('button', { name: 'Refresh history', exact: true }).click(); await expect(page.getByRole('alert')).toHaveCount(0); await page.getByRole('button', { name: 'Retry identity preparation', exact: true }).click(); await expect(page.getByText('Torrent identity ready', { exact: true })).toBeVisible(); expect(attempts).toBe(2);
});


test('submission confirmation recovers a lost queue response and reconciliation never uses submit', async ({ page }) => {
 let submission: { state: string; clientRevision: number; attempts: number; errorCode: string | null; updatedAt: string } | undefined;
 let revision = 3; let submits = 0; let reconciles = 0;
 const record = () => ({ ...saved, state: 'ready', revision, infoHash: '0123456789abcdef0123456789abcdef01234567', kind: 'magnet', submission });
 await page.route('**/api/v1/catalog/movies?**', route => route.fulfill({ json: { items: [movie], total: 1, offset: 0, limit: 25 } }));
 await page.route('**/api/v1/catalog/movies/7', route => route.fulfill({ json: movie }));
 await page.route('**/api/v1/source', route => route.fulfill({ json: { configured: true, name: 'Source', capabilities: { categories: [] } } }));
 await page.route('**/api/v1/download-client', route => route.fulfill({ json: { configured: true, revision: 2, url: 'http://localhost:9090' } }));
 await page.route('**/api/v1/settings', route => route.fulfill({ json: { revision: 4, directories: { downloadDirectory: '/data/downloads' } } }));
 await page.route('**/api/v1/acquisitions?**', route => route.fulfill({ json: { items: [record()], total: 1, limit: 25, offset: 0 } }));
 await page.route('**/api/v1/acquisitions/' + saved.id + '/submit', route => {
  submits++; expect(route.request().postDataJSON()).toEqual({ revision: 3, clientRevision: 2, settingsRevision: 4 });
  revision++; submission = { state: 'queued', clientRevision: 2, attempts: 0, errorCode: null, updatedAt: saved.updatedAt };
  return route.abort('failed');
 });
 await page.route('**/api/v1/acquisitions/' + saved.id + '/reconcile', route => {
  reconciles++; expect(route.request().postDataJSON()).toEqual({ revision });
  revision++; submission = { ...submission!, state: 'submitted', errorCode: null };
  return route.fulfill({ status: 202, json: record() });
 });
 await page.goto('/preview#/releases?movieId=7');
 await page.getByRole('button', { name: 'Submit to qBittorrent', exact: true }).click();
 const dialog = page.getByRole('dialog', { name: 'Submit to qBittorrent', exact: true });
 await expect(dialog.getByText('http://localhost:9090', { exact: false })).toBeVisible();
 await expect(dialog.getByText('/data/downloads', { exact: false })).toBeVisible();
 await expect(dialog.getByText('This may start a download.', { exact: false })).toBeVisible();
 await dialog.getByRole('button', { name: 'Cancel', exact: true }).click(); expect(submits).toBe(0);
 await page.getByRole('button', { name: 'Submit to qBittorrent', exact: true }).click();
 await expect(dialog.getByRole('button', { name: 'Confirm submission', exact: true })).toBeEnabled();
 await expect(dialog.getByText('/data/downloads', { exact: false })).toBeVisible();
 await page.screenshot({ path: 'test-results/submission-confirmation.png' });
 await dialog.getByRole('button', { name: 'Confirm submission', exact: true }).click();
 await expect(dialog.getByRole('alert')).toContainText('Refresh history');
 await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
 await expect(page.getByText('Submission queued', { exact: true })).toBeVisible();
 await expect(page.getByRole('button', { name: 'Cancel acquisition', exact: true })).toHaveCount(0);
 revision++; submission = { ...submission!, state: 'needs-review', attempts: 1, errorCode: 'download-client-authentication-failed' };
 await expect(page.getByText('qBittorrent rejected authentication. Check credentials and WebUI access restrictions.', { exact: true })).toBeVisible({ timeout: 6000 });
 expect(submits).toBe(1);
 revision++; submission = { ...submission!, state: 'uncertain', attempts: 1, errorCode: 'submission-uncertain' };
 await page.getByRole('button', { name: 'Refresh history', exact: true }).click();
 await expect(page.getByText('Submission uncertain', { exact: true })).toBeVisible({ timeout: 6000 });
 await page.getByRole('button', { name: 'Português (Brasil)', exact: true }).click();
 await expect(page.getByText('Envio incerto', { exact: true })).toBeVisible();
 await page.setViewportSize({ width: 390, height: 844 }); await page.evaluate(() => window.scrollTo(0, 0)); await page.screenshot({ path: 'test-results/submission-history-mobile.png', fullPage: true });
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
 await page.getByRole('button', { name: 'Conciliar novamente', exact: true }).click();
 await expect(page.getByText('Enviado ao qBittorrent', { exact: true })).toBeVisible(); expect(submits).toBe(1); expect(reconciles).toBe(1);
});
