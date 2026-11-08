import { useEffect, useState } from 'react';
import { api, ApiError } from './api';
import { useUI } from './ui';

type SearchCaps = { available: boolean; supportedParams: string[] };
type Caps = { search: SearchCaps; movieSearch: SearchCaps; maxLimit: number | null; defaultLimit: number | null; categories: { id: number; name: string }[] };
type Source = { id?: number; active?: boolean; kind?: 'torznab' | '1337x'; configured: boolean; revision: number; name: string; url: string; hasApiKey: boolean; capabilities: Caps | null; checkedAt: string | null };
type SourceList = { activeSourceId: number | null; items: Source[] };

export function SourceSettings() {
  const { t, locale } = useUI();
  const [sources, setSources] = useState<Source[]>([]);
  const [selectedId, setSelectedId] = useState<number>();
  const [creating, setCreating] = useState(false);
  const [saved, setSaved] = useState<Source>();
  const [kind, setKind] = useState<'torznab' | '1337x'>('torznab');
  const [name, setName] = useState('');
  const [url, setURL] = useState('');
  const [key, setKey] = useState('');
  const [clear, setClear] = useState(false);
  const [busy, setBusy] = useState<'save' | 'test' | 'activate'>();
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [conflict, setConflict] = useState(false);

  function explain(value: unknown) {
    if (value instanceof ApiError) {
      switch (value.code) {
        case 'source-changed': return t('Source changed in another request. Reload source before continuing.');
        case 'source-exists': return t('A source with this endpoint already exists.');
        case 'source-validation-failed': return t('Check the name and endpoint. Use a new key or explicitly clear it when changing endpoints.');
        case 'source-authentication-failed': return t('The source rejected its credentials. Check the API key.');
        case 'source-timeout': return t('The source timed out. Check its address and connectivity.');
        case 'source-rate-limited': return t('The source is rate limiting requests. Wait before testing again.');
        case 'source-challenge-required': return t('The source requires browser verification. Check the source in Settings.');
        case 'source-browser-unavailable': return t('Install or repair the local browser runtime for this source.');
        case 'source-invalid-response': return t('The source returned an invalid response.');
        case 'source-test-in-progress': return t('A source test is already running. Try again shortly.');
        case 'source-unconfigured': return t('Save a source before testing it.');
      }
    }
    return t('Source configuration is unavailable. Check connectivity or retry later.');
  }

  function load(value: Source) {
    setSaved(value); setSelectedId(value.id); setKind(value.kind ?? 'torznab'); setName(value.name); setURL(value.url);
    setKey(''); setClear(false); setConflict(false); setCreating(false);
  }

  function beginCreate() {
    setCreating(true); setSaved(undefined); setSelectedId(undefined); setKind('torznab'); setName(''); setURL('');
    setKey(''); setClear(false); setConflict(false); setError(''); setMessage('');
  }

  async function reload(preferred?: number) {
    setError(''); setMessage('');
    try {
      const list = await api<SourceList>('/sources');
      setSources(list.items);
      const chosen = list.items.find(item => item.id === (preferred ?? selectedId)) ?? list.items.find(item => item.id === list.activeSourceId);
      if (chosen) load(chosen); else beginCreate();
    } catch (value) { setError(explain(value)); }
  }

  useEffect(() => {
    const controller = new AbortController();
    api<SourceList>('/sources', { signal: controller.signal }).then(list => {
      setSources(list.items);
      const active = list.items.find(item => item.id === list.activeSourceId);
      if (active) load(active); else beginCreate();
    }).catch(value => { if (!controller.signal.aborted) setError(explain(value)); });
    return () => controller.abort();
  }, []);

  function select(id: number) {
    const source = sources.find(item => item.id === id);
    if (source) load(source);
  }

  const active = !!saved?.active;
  const dirty = creating
    ? !!(name.trim() || url.trim() || key.trim())
    : !!saved && active && (kind !== (saved.kind ?? 'torznab') || name.trim() !== saved.name || url.trim() !== saved.url || !!key.trim() || clear);

  async function save() {
    if (busy || (!creating && !saved)) return;
    setBusy('save'); setError(''); setMessage('');
    try {
      if (creating) {
        const created = await api<Source>('/sources', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Piratarr-Request': '1' }, body: JSON.stringify({ kind, name, url, apiKey: key }) });
        if (sources.length === 0 && created.id) {
          await api<Source>(`/sources/${created.id}/active`, { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-Piratarr-Request': '1' }, body: JSON.stringify({ revision: created.revision }) });
          await reload(created.id); setMessage(t('Source saved. Test it to discover capabilities.'));
        } else {
          await reload(created.id); setMessage(t('Source added. Activate it before testing or searching.'));
        }
      } else if (saved) {
        const value = await api<Source>('/source', { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-Piratarr-Request': '1' }, body: JSON.stringify({ revision: saved.revision, kind, name, url, apiKey: key, clearApiKey: clear }) });
        load(value); await reload(value.id); setMessage(t('Source saved. Test it to discover capabilities.'));
      }
    } catch (value) { setError(explain(value)); if (value instanceof ApiError && value.code === 'source-changed') setConflict(true); }
    finally { setKey(''); setBusy(undefined); }
  }

  async function activate() {
    if (!saved?.id || saved.active || busy) return;
    setBusy('activate'); setError(''); setMessage('');
    try {
      const value = await api<Source>(`/sources/${saved.id}/active`, { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-Piratarr-Request': '1' }, body: JSON.stringify({ revision: saved.revision }) });
      await reload(value.id); setMessage(t('Source activated. Existing stale search observations were invalidated.'));
    } catch (value) { setError(explain(value)); if (value instanceof ApiError && value.code === 'source-changed') setConflict(true); }
    finally { setBusy(undefined); }
  }

  async function test() {
    if (!saved || busy || dirty || conflict || !active) return;
    setBusy('test'); setError(''); setMessage('');
    try {
      const result = await api<{ connected: boolean; sourceRevision: number; capabilities: Caps; checkedAt: string }>('/source/test');
      setSaved({ ...saved, capabilities: result.capabilities, checkedAt: result.checkedAt });
      setMessage(t(kind === '1337x' ? 'Direct source page verified. Availability may change.' : 'Capabilities received. Search authentication is not verified by this test.'));
    } catch (value) { setError(explain(value)); if (value instanceof ApiError && value.code === 'source-changed') setConflict(true); }
    finally { setBusy(undefined); }
  }

  return <section className="source-settings" aria-labelledby="source-heading">
    <h3 id="source-heading" aria-label={t('Release source')}>{t('Release sources')}</h3>
    <p className="hint">{t('Save multiple sources and explicitly choose which one Lôro uses for search.')}</p>
    {error && <p role="alert">{error}</p>}{message && <p role="status">{message}</p>}
    <div className="source-list" aria-label={t('Saved source')}>
      {sources.map(source => <button key={source.id} type="button" aria-pressed={selectedId === source.id} className={selectedId === source.id ? 'active' : ''} disabled={!!busy} onClick={() => select(source.id!)}><span>{source.active && <span aria-hidden="true">● </span>}{source.name}</span><small>{source.kind === '1337x' ? '1337x' : 'Torznab'} · {source.active ? t('Active source') : t('Inactive source')}</small></button>)}
      <button type="button" aria-pressed={creating} className={creating ? 'active' : ''} disabled={!!busy} onClick={beginCreate}>{t('Add source')}</button>
    </div>
    {saved && !creating && <p className="hint">{t(saved.active ? 'Active source' : 'Inactive source')} · {t(saved.hasApiKey ? 'API key saved' : 'No API key saved')}</p>}
    <p className="hint">{t(kind === '1337x' ? 'Use the site origin only. Lôro accesses this source directly with its local browser runtime.' : 'Enter a Torznab endpoint without query parameters. Keep its API key in the separate field.')}</p>
    <form onSubmit={event => { event.preventDefault(); void save(); }}>
      <fieldset disabled={!!busy || conflict || (!creating && !active)}>
        <label htmlFor="source-kind">{t('Source type')}</label><select id="source-kind" value={kind} onChange={event => { setKind(event.target.value as 'torznab' | '1337x'); setKey(''); setClear(!creating); }}><option value="torznab">Torznab</option><option value="1337x">{t('1337x (direct)')}</option></select><p className="hint">{t('Each saved item is an indexer. Source type only selects its connection adapter.')}</p>
        <label htmlFor="source-name">{t('Source name')}</label><input id="source-name" value={name} onChange={event => setName(event.target.value)} maxLength={100} required autoComplete="off" />
        <label htmlFor="source-url">{t('Endpoint URL')}</label><input id="source-url" type="url" value={url} onChange={event => setURL(event.target.value)} maxLength={2048} required autoComplete="off" spellCheck={false} />
        <label htmlFor="source-key">{t('Source API key')}</label><input id="source-key" type="password" value={key} disabled={clear || kind === '1337x'} onChange={event => setKey(event.target.value)} maxLength={1024} autoComplete="new-password" spellCheck={false} />
        {!creating && <><p className="hint">{t('Leave blank to preserve the saved key. Endpoint changes need a new key or explicit clearing.')}</p><label className="source-clear"><input type="checkbox" checked={clear} onChange={event => { setClear(event.target.checked); setKey(''); }} />{t('Clear saved API key')}</label></>}
        <div className="actions"><button disabled={!dirty}>{t(busy === 'save' ? 'Saving…' : 'Save source')}</button><button type="button" className="secondary" disabled={!saved?.configured || !active || dirty} onClick={() => void test()}>{t(busy === 'test' ? 'Testing source…' : 'Test source')}</button></div>
      </fieldset>
    </form>
    {saved && !saved.active && !creating && <button type="button" disabled={!!busy || conflict} onClick={() => void activate()}>{t(busy === 'activate' ? 'Activating…' : 'Use this source')}</button>}
    {busy === 'test' && kind === '1337x' && <p role="status">{t('Open Logs to follow browser verification.')}</p>}
    {dirty && !creating && <p className="hint">{t('Save source changes before testing.')}</p>}
    {saved?.capabilities && <div className="source-capabilities"><h4>{t('Last successful capability discovery')}</h4><p>{saved.checkedAt ? new Date(saved.checkedAt).toLocaleString(locale) : ''}</p><p>{t('Movie search')}: {t(saved.capabilities.movieSearch.available ? 'Available' : 'Not advertised')} · {saved.capabilities.movieSearch.supportedParams.join(', ')}</p><p>{t('Text search')}: {t(saved.capabilities.search.available ? 'Available' : 'Not advertised')} · {saved.capabilities.search.supportedParams.join(', ')}</p><p>{t('Maximum results')}: {saved.capabilities.maxLimit ?? t('Unknown')}</p><ul>{saved.capabilities.categories.map(category => <li key={category.id}>{category.id} · {category.name}</li>)}</ul>{kind === 'torznab' && <p className="hint">{t('Capabilities may be public even when searches require an API key.')}</p>}</div>}
    {conflict && <button type="button" className="secondary" disabled={!!busy} onClick={() => void reload()}>{t('Reload source')}</button>}
  </section>;
}
