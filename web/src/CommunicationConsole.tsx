import { useEffect, useRef, useState } from 'react';
import { api } from './api';
import type { MessageKey } from './messages';
import { useUI } from './ui';

type Entry = { id: string; timestamp: string; requestId: string; integration: string; operation: string; phase: string; level: string; status: number | null; elapsedMs: number | null; retryAfterSeconds: number | null; code: string | null; count: number | null };
type Page = { entries: Entry[]; cursor: string; reset: boolean; truncated: boolean; storageAvailable: boolean };
const phaseNames: Record<string, MessageKey> = {
 'request.started': 'Request started', 'response.received': 'Response received', 'request.failed': 'Request failed',
 'operation.started': 'Operation started', 'operation.completed': 'Operation completed', 'operation.failed': 'Operation failed',
 'browser.started': 'Browser started', 'browser.navigation': 'Opening source page', 'browser.response': 'Browser response',
 'browser.challenge.detected': 'Cloudflare challenge detected', 'browser.widget.found': 'Verification widget',
 'browser.interaction': 'Verification interaction attempted', 'browser.challenge.cleared': 'Challenge cleared',
 'browser.parsed': 'Source results parsed', 'browser.stopped': 'Browser stopped', 'browser.failed': 'Browser failed', 'storage.degraded': 'Log storage unavailable',
};
const integrations = ['api', 'metadata', 'source', 'torrent', 'download-client', 'browser', 'jobs', 'system'];
export function CommunicationConsole() {
 const { t, locale } = useUI();
 const [open, setOpen] = useState(false); const [paused, setPaused] = useState(false); const [follow, setFollow] = useState(true);
 const [integration, setIntegration] = useState(''); const [entries, setEntries] = useState<Entry[]>([]); const [loading, setLoading] = useState(false);
 const [failed, setFailed] = useState(false); const [storage, setStorage] = useState(true); const [truncated, setTruncated] = useState(false);
 const cursor = useRef(''); const panel = useRef<HTMLElement>(null); const list = useRef<HTMLOListElement>(null); const toggle = useRef<HTMLButtonElement>(null);
 function close() { setOpen(false); toggle.current?.focus(); }
 useEffect(() => { if (open) panel.current?.focus(); }, [open]);
 useEffect(() => {
  if (!open || paused) return;
  const controller = new AbortController(); let timer: ReturnType<typeof setTimeout>;
  async function poll() {
   setLoading(true);
   const params = new URLSearchParams({ limit: '100' }); if (cursor.current) params.set('after', cursor.current); if (integration) params.set('integration', integration);
   try {
    const page = await api<Page>('/system/communications?' + params, { signal: controller.signal });
    if (controller.signal.aborted) return;
    cursor.current = page.cursor; setStorage(page.storageAvailable); setTruncated(page.truncated); setFailed(false);
    setEntries(old => { const values = page.reset ? page.entries : [...old, ...page.entries]; return [...new Map(values.map(item => [item.id, item])).values()].slice(-500); });
   } catch { if (!controller.signal.aborted) setFailed(true); }
   finally { if (!controller.signal.aborted) { setLoading(false); timer = setTimeout(() => void poll(), 1500); } }
  }
  void poll(); return () => { controller.abort(); clearTimeout(timer); };
 }, [open, paused, integration]);
 useEffect(() => { if (follow && list.current) list.current.scrollTop = list.current.scrollHeight; }, [entries, follow]);
 return <>
  <button ref={toggle} className="communication-toggle secondary" type="button" aria-expanded={open} aria-controls="communication-console" title={t('Communication logs')} onClick={() => setOpen(value => !value)}>
   <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true"><path d="M4 5h16v14H4zM7 9l3 3-3 3m6 0h4" /></svg><span className="navigation-label">{t('Logs')}</span>
  </button>
  {open && <section ref={panel} id="communication-console" className="communication-console" role="region" aria-label={t('Communication logs')} tabIndex={-1} onKeyDown={event => { if (event.key === 'Escape') close(); }}>
   <div className="communication-toolbar"><h2>{t('Communication logs')}</h2><button type="button" className="secondary" onClick={close}>{t('Close logs')}</button></div>
   <div className="communication-controls"><label>{t('Integration')}<select value={integration} onChange={event => { setIntegration(event.target.value); cursor.current = ''; setEntries([]); }}><option value="">{t('All integrations')}</option>{integrations.map(value => <option key={value} value={value}>{value}</option>)}</select></label><button type="button" className="secondary" onClick={() => setPaused(value => !value)}>{t(paused ? 'Resume logs' : 'Pause logs')}</button><label className="communication-follow"><input type="checkbox" checked={follow} onChange={event => setFollow(event.target.checked)} />{t('Follow new events')}</label></div>
   {!storage && <p role="status">{t('Log storage unavailable. Showing temporary history.')}</p>}{truncated && <p role="status">{t('Some older events are no longer retained.')}</p>}{failed && <p role="alert">{t('Communication logs are unavailable. Retrying…')}</p>}
   <ol ref={list} className="communication-events" aria-label={t('Communication events')}>{entries.map(entry => <li key={entry.id} className={'communication-event ' + entry.level}><details><summary><time dateTime={entry.timestamp}>{new Date(entry.timestamp).toLocaleTimeString(locale)}</time><span>{entry.integration} · {t(phaseNames[entry.phase] ?? 'Operation completed')}</span><strong>{entry.status ?? entry.code ?? '—'}</strong>{entry.elapsedMs !== null && <small>{entry.elapsedMs.toLocaleString(locale)} ms</small>}</summary><dl><dt>{t('Operation')}</dt><dd>{entry.operation}</dd><dt>{t('Correlation ID')}</dt><dd><code>{entry.requestId}</code></dd>{entry.code && <><dt>{t('Diagnostic code')}</dt><dd>{entry.code}</dd></>}{entry.count !== null && <><dt>{t('Count')}</dt><dd>{entry.count}</dd></>}{entry.retryAfterSeconds !== null && <><dt>{t('Retry after')}</dt><dd>{entry.retryAfterSeconds} s</dd></>}</dl></details></li>)}</ol>
   {!entries.length && <p role="status">{t(loading ? 'Loading communication logs…' : 'No communication events yet.')}</p>}
   <p className="communication-note">{t(paused ? 'Log display paused. Recording continues.' : 'Sanitized application requests and responses. Credentials and raw bodies are omitted.')}</p>
  </section>}
 </>;
}
