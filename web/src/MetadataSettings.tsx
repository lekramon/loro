import { useEffect, useState } from 'react';
import { metadataError } from './metadataErrors';
import { api } from './api';
import { useUI } from './ui';

type Status = { configured: boolean };
export function MetadataSettings() {
  const { t } = useUI();
  const [configured, setConfigured] = useState<boolean>();
  const [token, setToken] = useState('');
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState('');
  const [message, setMessage] = useState<'saved' | 'error'>();
  useEffect(() => {
    const controller = new AbortController();
    api<Status>('/metadata/status', { signal: controller.signal }).then(value => setConfigured(value.configured)).catch(() => {});
    return () => controller.abort();
  }, []);
  async function save() {
    if (!token.trim() || saving || testing) return;
    setTestResult('');
    setSaving(true); setMessage(undefined);
    try {
      const result = await api<Status>('/metadata/credential', { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-Piratarr-Request': '1' }, body: JSON.stringify({ token }) });
      setConfigured(result.configured); setToken(''); setMessage('saved');
    } catch { setToken(''); setMessage('error'); }
    finally { setSaving(false); }
  }
  async function testConnection() {
    if (saving || testing || token.trim()) return;
    setTesting(true); setTestResult('');
    try { await api('/metadata/test'); setTestResult(t('TMDB connection successful.')); }
    catch (error) { setTestResult(metadataError(error, t)); }
    finally { setTesting(false); }
  }
  return <section className="metadata-settings" aria-labelledby="metadata-heading">
    <h3 id="metadata-heading">{t('Movie metadata')}</h3>
    <p className="hint" role="status">{t(configured === undefined ? 'Checking metadata configuration…' : configured ? 'TMDB token configured' : 'TMDB token not configured')}</p>
    <label htmlFor="tmdb-token">{t('TMDB API Read Access Token')}</label>
    <input id="tmdb-token" type="password" onKeyDown={event => { if (event.key === 'Enter') { event.preventDefault(); void save(); } }} value={token} onChange={event => { setToken(event.target.value); setMessage(undefined); setTestResult(''); }} autoComplete="new-password" spellCheck={false} maxLength={8192} disabled={saving || testing} />
    <p className="hint">{t('Stored only on this service. Leave blank to keep the current token. Saving does not test TMDB authentication.')}</p>
    <button type="button" disabled={!token.trim() || saving || testing} onClick={() => void save()}>{t(saving ? 'Saving…' : 'Save TMDB token')}</button>
    <button type="button" className="secondary" disabled={saving || testing || !!token.trim()} onClick={() => void testConnection()}>{t(testing ? 'Testing connection…' : 'Test connection')}</button>
    {token.trim() && <p className="hint">{t('Save your changes before testing the saved token.')}</p>}
    {testResult && <p role="status">{testResult}</p>}
    <a className="metadata-help" href="/search">{t('Search movies on TMDB')}</a>
    {message && <p role={message === 'error' ? 'alert' : 'status'}>{t(message === 'saved' ? 'Token saved. Test the connection to verify access.' : 'Unable to save token. Check the Read Access Token and try again.')}</p>}
    <a className="metadata-help" href="https://www.themoviedb.org/settings/api" target="_blank" rel="noreferrer">{t('Open TMDB API settings')}</a>
  </section>;
}
