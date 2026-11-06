import { useEffect, useRef, useState } from 'react';
import { api, ApiError } from './api';
import type { Candidate } from './ReleaseSearch';
import type { MessageKey } from './messages';
import { useUI } from './ui';

type Acquisition = { id: string; movieId: number; movieTitle: string; candidate: Candidate; revision: number; state: 'selected' | 'preparing' | 'ready' | 'needs-review' | 'cancelled'; infoHash: string | null; kind: string | null; reviewCode: string | null; overrideConfirmed: boolean; updatedAt: string };
type History = { items: Acquisition[]; total: number; limit: number; offset: number };
const headers = { 'Content-Type': 'application/json', 'X-Piratarr-Request': '1' };
const states = { selected: 'Selection saved', preparing: 'Preparing identity…', ready: 'Torrent identity ready', 'needs-review': 'Needs review', cancelled: 'Cancelled' } as const;
function errorText(value: unknown, t: (key: MessageKey) => string) {
  if (value instanceof ApiError) switch (value.code) {
    case 'observation-expired': case 'observation-not-found': return t('This release observation expired. Search again before selecting.');
    case 'observation-changed': return t('Source or preferences changed. Search again and review the candidate.');
    case 'profile-confirmation-required': return t('Confirm the profile differences and unknown attributes for this acquisition.');
    case 'idempotency-conflict': return t('This selection request conflicts with an earlier choice. Close and select again.');
    case 'acquisition-changed': case 'acquisition-state-conflict': case 'preparation-in-progress': return t('Acquisition changed or preparation is running. Refresh before retrying.');
  }
  return t('Unable to update the acquisition. Refresh history before retrying an uncertain response.');
}
function reviewText(code: string, t: (key: MessageKey) => string) {
  switch (code) {
    case 'source-changed': return t('Source configuration changed. Cancel this choice and search again.');
    case 'preparation-interrupted': return t('Preparation was interrupted. Retry identity preparation when ready.');
    case 'source-authentication-failed': return t('The source rejected its credentials. Check the API key.');
    case 'source-timeout': return t('The source timed out. Check its address and connectivity.');
    case 'source-rate-limited': return t('The source is rate limiting requests. Wait before searching again.');
    case 'source-unavailable': return t('The source is unavailable. Retry identity preparation later.');
    case 'identity-mismatch': return t('Torrent identity evidence conflicts. Choose another result or review the source.');
    case 'unsupported-identity': return t('This identity is not supported yet. This stage supports v1 torrents only.');
    case 'torrent-invalid': case 'torrent-too-large': return t('The source did not return valid supported torrent metadata.');
    default: return t('The release has no supported unambiguous download reference. Choose another result.');
  }
}

export function ReleaseSelection({ candidate, movieTitle, onClose, onSaved }: { candidate: Candidate; movieTitle: string; onClose: () => void; onSaved: () => void }) {
  const { t } = useUI(); const dialog = useRef<HTMLDialogElement>(null);
  const [confirmed, setConfirmed] = useState(false); const [busy, setBusy] = useState(false); const [failure, setFailure] = useState<unknown>();
  const [key] = useState(() => crypto.randomUUID()); const [intent, setIntent] = useState<Acquisition>();
  const override = candidate.status !== 'match';
  useEffect(() => { dialog.current?.showModal(); }, []);
  async function prepare() {
    if (busy || override && !confirmed) return;
    setBusy(true); setFailure(undefined); let current = intent;
    try {
      if (!current) { const result = await api<{ acquisition: Acquisition }>('/acquisitions', { method: 'POST', headers, body: JSON.stringify({ observationId: candidate.observationId, idempotencyKey: key, confirmOverride: confirmed }) }); current = result.acquisition; setIntent(current); onSaved(); }
      if (current.state === 'selected' || current.state === 'needs-review') await api<Acquisition>('/acquisitions/' + current.id + '/prepare', { method: 'POST', headers, body: JSON.stringify({ revision: current.revision }) });
      onSaved(); onClose();
    } catch (value) {
      setFailure(value); onSaved();
      if (current) try { setIntent(await api<Acquisition>('/acquisitions/' + current.id)); } catch { /* History refresh remains available after an uncertain response. */ }
    } finally { setBusy(false); }
  }
  return <dialog className="file-review-dialog acquisition-dialog" ref={dialog} aria-labelledby="selection-heading" onCancel={event => { event.preventDefault(); if (!busy) onClose(); }}>
    <h2 id="selection-heading">{t('Select release')}</h2><p>{movieTitle}</p><p className="review-path">{candidate.title}</p><p className="hint">{t('This saves your choice and prepares torrent identity. It does not start a download.')}</p>
    <ul className="selection-checks">{candidate.checks.map(check => <li key={check.dimension}>{t(({ resolution: 'Resolution', format: 'Release format', container: 'File container', audio: 'Audio' } as const)[check.dimension])}: {check.dimension === 'audio' ? candidate.audioLanguages.join(', ') || t('Unknown') : candidate[check.dimension] ?? t('Unknown')} · {t(({ match: 'Accepted', mismatch: 'Outside profile', unknown: 'Unknown' } as const)[check.status])}<small>{t('Profile')}: {check.accepted.map(value => value === 'original' ? t('Original audio') : value).join(', ')}</small></li>)}</ul>
    {failure !== undefined && <p role="alert">{errorText(failure, t)}</p>}
    <form onSubmit={event => { event.preventDefault(); void prepare(); }}><fieldset disabled={busy}>
      {override && <label className="identity-confirmation"><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} />{t('I accept these differences or unknown attributes for this acquisition only.')}</label>}
      <div className="actions"><button disabled={override && !confirmed}>{t(busy ? 'Preparing identity…' : 'Prepare acquisition')}</button><button type="button" className="secondary" onClick={onClose}>{t('Cancel')}</button></div>
    </fieldset></form>
  </dialog>;
}

export function AcquisitionHistory({ movieId, version }: { movieId: number; version: number }) {
  const { t, locale } = useUI(); const [loading, setLoading] = useState(true); const [history, setHistory] = useState<History>(); const [offset, setOffset] = useState(0); const [refresh, setRefresh] = useState(0); const [busy, setBusy] = useState(false); const [failure, setFailure] = useState<unknown>();
  useEffect(() => {
    const controller = new AbortController(); setLoading(true);
    void api<History>('/acquisitions?' + new URLSearchParams({ movieId: String(movieId), limit: '25', offset: String(offset) }), { signal: controller.signal }).then(value => { if (!controller.signal.aborted) setHistory(value); }).catch(value => { if (!controller.signal.aborted) setFailure(value); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [movieId, version, offset, refresh]);
  async function action(value: Acquisition, kind: 'prepare' | 'cancel') {
    setBusy(true); setFailure(undefined);
    try { await api<Acquisition>('/acquisitions/' + value.id + '/' + kind, { method: 'POST', headers, body: JSON.stringify({ revision: value.revision }) }); }
    catch (error) { setFailure(error); }
    finally { setLoading(true); setRefresh(value => value + 1); setBusy(false); }
  }
  return <section className="acquisition-history" aria-labelledby="history-heading"><div className="release-heading"><h2 id="history-heading">{t('Saved acquisitions')}</h2><button className="secondary" disabled={busy || loading} onClick={() => { setFailure(undefined); setRefresh(value => value + 1); }}>{t('Refresh history')}</button></div><p className="hint">{t('Prepared choices only. Downloads are not started in this stage.')}</p>
    {failure !== undefined && <p role="alert">{errorText(failure, t)}</p>}
    {!history ? <p role="status">{t('Loading…')}</p> : <>{!history.items.length && <p>{t('No saved acquisitions for this movie.')}</p>}<div className="release-list">{history.items.map(value => <article className="release-card" key={value.id}><h3>{value.candidate.title}</h3><p className="acquisition-state">{t(states[value.state])}</p><p className="hint">{new Date(value.updatedAt).toLocaleString(locale)}{value.overrideConfirmed && <> · {t('Profile override confirmed for this choice')}</>}</p>{value.infoHash && <p className="torrent-identity"><span>{t('Torrent identity')}: </span><code>{value.infoHash}</code></p>}{value.reviewCode && <p>{reviewText(value.reviewCode, t)}</p>}<div className="actions">{(value.state === 'selected' || value.state === 'needs-review') && <button disabled={busy || loading} onClick={() => void action(value, 'prepare')}>{t('Retry identity preparation')}</button>}{value.state !== 'cancelled' && <button className="secondary" disabled={busy || loading} onClick={() => void action(value, 'cancel')}>{t('Cancel acquisition')}</button>}</div></article>)}</div><div className="actions"><button className="secondary" disabled={busy || loading || offset === 0} onClick={() => setOffset(Math.max(0, offset - 25))}>{t('Previous acquisitions')}</button><button className="secondary" disabled={busy || loading || offset + history.limit >= history.total} onClick={() => setOffset(offset + 25)}>{t('Next acquisitions')}</button></div></>}
  </section>;
}
