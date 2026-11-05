import { expect, test } from '@playwright/test';
import { createServer } from 'node:http';
import { mkdtemp, mkdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

test('qBittorrent settings keep passwords private and test fresh real simulated sessions', async ({ page, request }) => {
  const secret = 'dummy-browser-qbit-password'; const sid = 'dummy-browser-session'; let logins = 0; let logouts = 0; let forbidden = false;
  const server = createServer(async (req, res) => {
    const address = server.address(); if (!address || typeof address === 'string') throw new Error('missing port');
    expect(req.headers.origin).toBe(`http://127.0.0.1:${address.port}`); expect(req.headers.referer).toBe(`http://127.0.0.1:${address.port}/qbit/`);
    res.setHeader('Content-Type', 'text/plain');
    if (req.url === '/qbit/api/v2/auth/login') {
      logins++; expect(req.method).toBe('POST'); expect(req.headers.cookie).toBeUndefined(); let body = ''; for await (const chunk of req) body += chunk;
      const form = new URLSearchParams(body); expect(form.get('username')).toBe('admin'); expect(form.get('password')).toBe(secret);
      if (forbidden) { res.end('Fails.'); return; } res.setHeader('Set-Cookie', `SID=${sid}; Path=/qbit; HttpOnly`); res.end('Ok.'); return;
    }
    expect(req.headers.cookie).toBe(`SID=${sid}`);
    if (req.url === '/qbit/api/v2/app/version') { expect(req.method).toBe('GET'); res.end('v5.1.0'); return; }
    if (req.url === '/qbit/api/v2/app/webapiVersion') { res.end('2.11.3'); return; }
    if (req.url === '/qbit/api/v2/auth/logout') { logouts++; expect(req.method).toBe('POST'); res.end(); return; }
    throw new Error('unexpected request or torrent mutation');
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve)); const address = server.address(); if (!address || typeof address === 'string') throw new Error('missing port'); let root: string | undefined;
  try {
    const setup = await (await request.get('/api/v1/setup')).json();
    if (!setup.completed) { root = await mkdtemp(join(tmpdir(), 'piratarr-client-browser-')); const downloads = join(root, 'downloads'), movies = join(root, 'movies'); await mkdir(downloads); await mkdir(movies); const configured = await request.post('/api/v1/setup', { headers: { 'X-Piratarr-Request': '1' }, data: { directories: { downloadDirectory: downloads, movieDirectory: movies }, profile: { acceptedResolutions: ['1080p'], preferredResolution: '1080p', audioPreferences: ['original'] } } }); expect(configured.ok()).toBe(true); }
    await page.goto('/settings'); const panel = page.locator('section[aria-labelledby="download-client-heading"]'); await expect(panel).toBeVisible();
    await panel.getByLabel('qBittorrent WebUI URL', { exact: true }).fill(`http://127.0.0.1:${address.port}/qbit/`); await panel.getByLabel('qBittorrent username', { exact: true }).fill('admin'); await panel.getByLabel('qBittorrent password', { exact: true }).fill(secret); await expect(panel.getByRole('button', { name: 'Test download client', exact: true })).toBeDisabled();
    await panel.getByRole('button', { name: 'Save download client', exact: true }).click(); await expect(panel.getByText('Download client saved. Test the connection.', { exact: true })).toBeVisible(); expect(logins).toBe(0); await expect(panel.getByLabel('qBittorrent password', { exact: true })).toHaveValue(''); expect(await (await request.get('/api/v1/download-client')).text()).not.toContain(secret); expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toContain(secret);
    await panel.getByRole('button', { name: 'Test download client', exact: true }).click(); await expect(panel.getByText('qBittorrent authentication and versions verified.', { exact: true })).toBeVisible(); expect(logins).toBe(1); expect(logouts).toBe(1); await page.reload(); await expect(panel.getByText('qBittorrent v5.1.0 · WebUI API 2.11.3', { exact: true })).toBeVisible(); await expect(panel.getByLabel('qBittorrent password', { exact: true })).toHaveValue('');
    await panel.getByRole('button', { name: 'Test download client', exact: true }).click(); await expect(panel.getByText('qBittorrent authentication and versions verified.', { exact: true })).toBeVisible(); expect(logins).toBe(2); expect(logouts).toBe(2);
    await panel.getByLabel('qBittorrent username', { exact: true }).fill('other-user'); await panel.getByRole('button', { name: 'Save download client', exact: true }).click(); await expect(panel.getByRole('alert')).toContainText('require a new password'); expect(logins).toBe(2); await panel.getByRole('button', { name: 'Reload download client', exact: true }).click(); await expect(panel.getByLabel('qBittorrent username', { exact: true })).toHaveValue('admin');
    forbidden = true; await page.getByRole('button', { name: 'Português (Brasil)', exact: true }).click(); await panel.getByRole('button', { name: 'Testar cliente de download', exact: true }).click(); await expect(panel.getByRole('alert')).toContainText('O qBittorrent rejeitou a autenticação'); await page.getByRole('button', { name: 'English (US)', exact: true }).click(); await expect(panel.getByRole('alert')).toContainText('qBittorrent rejected authentication'); await page.getByRole('button', { name: 'Português (Brasil)', exact: true }).click(); await expect(panel.getByText('qBittorrent v5.1.0 · WebUI API 2.11.3', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Escuro', exact: true }).click(); await panel.scrollIntoViewIfNeeded(); await page.screenshot({ path: 'test-results/download-client-dark.png' }); await page.setViewportSize({ width: 390, height: 844 }); await page.getByRole('button', { name: 'Claro', exact: true }).click(); await panel.scrollIntoViewIfNeeded(); expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true); await page.screenshot({ path: 'test-results/download-client-mobile.png' });
  } finally { await new Promise<void>(resolve => server.close(() => resolve())); if (root && root.startsWith(join(tmpdir(), 'piratarr-client-browser-'))) await rm(root, { recursive: true, force: true }); }
});
