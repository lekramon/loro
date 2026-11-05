import { useEffect, useState } from 'react';
import { api, ApiError } from './api';
import { useUI } from './ui';

type Connection = { configured: boolean; revision: number; url: string; username: string; hasPassword: boolean; appVersion: string | null; apiVersion: string | null; checkedAt: string | null };
type Evidence = { connected: boolean; clientRevision: number; appVersion: string; apiVersion: string; checkedAt: string };
const headers = { 'Content-Type': 'application/json', 'X-Piratarr-Request': '1' };

export function DownloadClientSettings() {
  const { t, locale } = useUI();
  const [saved, setSaved] = useState<Connection>(); const [url, setURL] = useState(''); const [username, setUsername] = useState(''); const [password, setPassword] = useState('');
  const [busy, setBusy] = useState<'save' | 'test' | 'reload'>(); const [error, setError] = useState<unknown>(); const [message, setMessage] = useState<'saved' | 'tested'>(); const [conflict, setConflict] = useState(false);
  function explain(value: unknown) {
    if (value instanceof ApiError) switch (value.code) {
      case 'download-client-changed': return t('Download client changed. Reload before continuing.');
      case 'download-client-validation-failed': return t('Check the WebUI URL and username. First save or changes to either require a new password.');
      case 'download-client-authentication-failed': return t('qBittorrent rejected authentication. Check credentials and WebUI access restrictions.');
      case 'download-client-unsupported-version': return t('Use stable qBittorrent 5.x with WebUI API 2.x.');
      case 'download-client-invalid-response': return t('The address did not return a valid qBittorrent response.');
      case 'download-client-timeout': return t('qBittorrent timed out. Check its address and connectivity.');
      case 'download-client-rate-limited': return t('qBittorrent is rate limiting requests. Wait before testing again.');
      case 'download-client-unconfigured': return t('Save the download client before testing it.');
      case 'download-client-test-in-progress': return t('A download client test is running. Try again shortly.');
      case 'setup-required': return t('Complete setup before configuring a download client.');
    }
    return t('Unable to reach the download client configuration. Retry later.');
  }
  function load(value: Connection) { setSaved(value); setURL(value.url); setUsername(value.username); setPassword(''); setConflict(false); }
  useEffect(() => { const controller = new AbortController(); api<Connection>('/download-client', { signal: controller.signal }).then(load).catch(value => { if (!controller.signal.aborted) setError(value); }); return () => controller.abort(); }, []);
  const dirty = !!saved && (url.trim().replace(/\/+$/, '') !== saved.url || username.trim() !== saved.username || password !== '');
  async function reload() { setBusy('reload'); setError(undefined); setMessage(undefined); try { load(await api<Connection>('/download-client')); } catch (value) { setError(value); } finally { setBusy(undefined); } }
  async function save() {
    if (!saved || busy) return;
    setBusy('save'); setError(undefined); setMessage(undefined);
    try { load(await api<Connection>('/download-client', { method: 'PUT', headers, body: JSON.stringify({ revision: saved.revision, url, username, password }) })); setMessage('saved'); }
    catch (value) { setError(value); if (value instanceof ApiError && value.code === 'download-client-changed') setConflict(true); }
    finally { setPassword(''); setBusy(undefined); }
  }
  async function test() {
    if (!saved || busy || dirty || conflict) return;
    setBusy('test'); setError(undefined); setMessage(undefined);
    try { const result = await api<Evidence>('/download-client/test', { method: 'POST', headers, body: '{}' }); setSaved({ ...saved, appVersion: result.appVersion, apiVersion: result.apiVersion, checkedAt: result.checkedAt }); setMessage('tested'); }
    catch (value) { setError(value); if (value instanceof ApiError && value.code === 'download-client-changed') setConflict(true); }
    finally { setBusy(undefined); }
  }
  return <section className="source-settings" aria-labelledby="download-client-heading">
    <h3 id="download-client-heading">{t('Download client · qBittorrent')}</h3><p className="hint">{t('Enter the WebUI base URL, including a reverse-proxy path if needed. Testing does not start downloads.')}</p>
    {error !== undefined && <p role="alert">{explain(error)}</p>}{message && <p role="status">{t(message === 'saved' ? 'Download client saved. Test the connection.' : 'qBittorrent authentication and versions verified.')}</p>}
    {saved ? <><p className="hint">{t(saved.configured ? 'Download client configured' : 'No download client configured')} · {t(saved.hasPassword ? 'Password saved' : 'No password saved')}</p><form onSubmit={event => { event.preventDefault(); void save(); }}><fieldset disabled={!!busy || conflict}>
      <label htmlFor="client-url">{t('qBittorrent WebUI URL')}</label><input id="client-url" type="url" value={url} onChange={event => setURL(event.target.value)} maxLength={2048} required autoComplete="off" spellCheck={false} placeholder="http://127.0.0.1:8081" />
      <label htmlFor="client-user">{t('qBittorrent username')}</label><input id="client-user" value={username} onChange={event => setUsername(event.target.value)} maxLength={100} required autoComplete="off" spellCheck={false} />
      <label htmlFor="client-password">{t('qBittorrent password')}</label><input id="client-password" type="password" value={password} onChange={event => setPassword(event.target.value)} maxLength={1024} autoComplete="new-password" spellCheck={false} /><p className="hint">{t('Leave blank to preserve the password. Changing the URL or username requires a new password.')}</p>
      <div className="actions"><button disabled={!dirty}>{t(busy === 'save' ? 'Saving…' : 'Save download client')}</button><button type="button" className="secondary" disabled={!saved.configured || dirty} onClick={() => void test()}>{t(busy === 'test' ? 'Testing qBittorrent…' : 'Test download client')}</button></div>
    </fieldset></form>{dirty && <p className="hint">{t('Save download client changes before testing.')}</p>}
      {saved.checkedAt && <div className="source-capabilities"><h4>{t('Last successful connection test')}</h4><p>{new Date(saved.checkedAt).toLocaleString(locale)}</p><p>qBittorrent {saved.appVersion} · WebUI API {saved.apiVersion}</p><p className="hint">{t('Connection testing does not verify download permissions or folder mapping.')}</p></div>}
    </> : <p role="status">{t('Loading download client…')}</p>}
    <button type="button" className="secondary" disabled={!!busy} onClick={() => void reload()}>{t('Reload download client')}</button>
  </section>;
}
