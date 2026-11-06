import { useEffect, useState } from 'react';
import { api, ApiError } from './api';
import { useUI } from './ui';
import type { MessageKey } from './messages';

type Handoff = { version: 1; state: 'awaiting-validation' | 'needs-review'; reasonCode: string | null; file: { index: number; relativePath: string; sizeBytes: number } | null };
type Download = { acquisitionId: string; movieTitle: string; releaseTitle: string; infoHash: string; revision: number; state: 'pending' | 'queued' | 'downloading' | 'paused' | 'unavailable' | 'completed' | 'failed' | 'needs-review'; pollState: 'queued' | 'running' | 'idle'; nextCheckAt: string | null; progress: number | null; sizeBytes: number | null; remainingBytes: number | null; downloadSpeed: number | null; clientState: string | null; errorCode: string | null; lastCheckedAt: string | null; lastObservedAt: string | null; completedAt: string | null; handoff: Handoff | null; updatedAt: string };
type Page = { items: Download[]; total: number; limit: number; offset: number };
type Event = { state: Download['state']; code: string | null; progress: number | null; handoffState: string | null; createdAt: string };
type Detail = { download: Download; events: Event[]; eventTotal: number };
const states = { pending: 'Waiting for download check', queued: 'Queued', downloading: 'Downloading', paused: 'Paused', unavailable: 'Client unavailable', completed: 'Completed', failed: 'Download failed', 'needs-review': 'Needs review' } as const;
const reasons: Record<string, MessageKey> = {
 'download-torrent-missing': 'The torrent is missing from qBittorrent. Check it in the client before checking again.',
 'download-ownership-lost': 'Torrent ownership changed. Review its category and acquisition tag in qBittorrent.',
 'download-path-mismatch': 'Client paths differ from the saved download directory. Review the installation paths.',
 'download-file-path-unsafe': 'File paths are unsafe or ambiguous. Manual review is required.',
 'download-completion-changed': 'Completion evidence changed during the check. Check the download again.',
 'download-completion-inconsistent': 'qBittorrent completion fields are inconsistent. Manual review is required.',
 'download-state-unsupported': 'This transfer state is not supported. Check the download in qBittorrent.',
 'download-file-ambiguous': 'No single video file can be identified. File selection needs review.',
 'download-files-incomplete': 'Selected files are incomplete or their sizes disagree. File selection needs review.',
 'download-transfer-failed': 'qBittorrent reported a transfer error. Resolve it in the client before checking again.',
 'download-files-missing': 'qBittorrent reports missing files. Resolve it in the client before checking again.',
 'download-client-authentication-failed': 'qBittorrent rejected authentication. Check credentials and WebUI access restrictions.',
 'download-client-unsupported-version': 'Use stable qBittorrent 5.x with WebUI API 2.x.',
 'download-client-invalid-response': 'The address did not return a valid qBittorrent response.',
 'download-client-timeout': 'qBittorrent timed out. Check its address and connectivity.',
 'download-client-unavailable': 'qBittorrent is unavailable. Last observed progress is retained.',
 'download-client-rate-limited': 'qBittorrent is rate limiting requests. Wait before retrying.',
 'download-client-changed': 'Client or settings changed after confirmation. This saved job remains paused for review.',
 'submission-configuration-changed': 'Client or settings changed after confirmation. This saved job remains paused for review.',
};
function errorKey(error: unknown): MessageKey {
 if (error instanceof ApiError) switch (error.code) { case 'setup-required': return 'Complete setup before tracking downloads.'; case 'download-not-found': return 'No download task exists for this acquisition.'; case 'download-changed': return 'Download changed. Refresh before checking again.'; case 'download-check-cooldown': return 'A scheduled check is waiting. Refresh after its due time.'; }
 return 'Unable to read or update download tracking. Refresh before retrying.';
}
export function DownloadsPage({ acquisitionId }: { acquisitionId?: string }) {
 const { t, locale } = useUI(); const [list, setList] = useState<Page>(); const [detail, setDetail] = useState<Detail>(); const [offset, setOffset] = useState(0); const [refresh, setRefresh] = useState(0); const [loading, setLoading] = useState(true); const [busy, setBusy] = useState(false); const [readError, setReadError] = useState<unknown>(); const [operationError, setOperationError] = useState<unknown>();
 useEffect(() => { const controller = new AbortController(); setLoading(true); const target = acquisitionId ? '/downloads/' + acquisitionId : '/downloads?limit=25&offset=' + offset;
  void api<Page | Detail>(target, { signal: controller.signal }).then(value => { if (controller.signal.aborted) return; setReadError(undefined); setOperationError((current: unknown) => current instanceof ApiError ? current : undefined); if (acquisitionId) setDetail(value as Detail); else setList(value as Page); }).catch(value => { if (!controller.signal.aborted) setReadError(value); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
  return () => controller.abort();
 }, [acquisitionId, offset, refresh]);
 const items = acquisitionId ? (detail ? [detail.download] : []) : list?.items;
 const active = items?.some(value => value.pollState !== 'idle');
 useEffect(() => { if (!active) return; const timer = window.setInterval(() => { if (document.visibilityState === 'visible') setRefresh(value => value + 1); }, 2000); return () => window.clearInterval(timer); }, [active]);
 async function check(download: Download) { setBusy(true); setOperationError(undefined); setReadError(undefined); try { await api('/downloads/' + download.acquisitionId + '/check', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Piratarr-Request': '1' }, body: JSON.stringify({ revision: download.revision }) }); } catch (value) { setOperationError(value); } finally { setRefresh(value => value + 1); setBusy(false); } }
 const bytes = (value: number | null) => { if (value === null) return t('Unknown'); const power = value >= 1024 ** 3 ? 3 : value >= 1024 ** 2 ? 2 : value >= 1024 ? 1 : 0; return new Intl.NumberFormat(locale, { maximumFractionDigits: power ? 1 : 0 }).format(value / 1024 ** power) + ' ' + ['B', 'KiB', 'MiB', 'GiB'][power]; };
 const time = (value: string | null) => value ? new Date(value).toLocaleString(locale) : t('Unknown');
 const error = readError ?? operationError;
 return <section className="demo-content tracking-page"><div className="release-heading"><h1>{t('Downloads')}</h1><button className="secondary" disabled={loading || busy} onClick={() => { setReadError(undefined); setOperationError(undefined); setRefresh(value => value + 1); }}>{t('Refresh downloads')}</button></div>
 <p className="hint">{t('Transfer observations from qBittorrent. Checks do not restart transfers or import files.')}</p>{acquisitionId && <p><a href="#/downloads">{t('All downloads')}</a></p>}
 {error !== undefined && <p role="alert">{t(errorKey(error))}{error instanceof ApiError && error.code === 'setup-required' && <> <a href="/setup">{t('Setup')}</a></>}</p>}
 {!items && loading && <p role="status">{t('Loading…')}</p>}{items?.length === 0 && <p>{t('No tracked downloads yet. Submit a prepared acquisition from Releases.')}</p>}
 <div className="release-list">{items?.map(value => <article className="release-card tracking-card" key={value.acquisitionId}><h2><a href={'#/downloads/' + value.acquisitionId}>{value.movieTitle}</a></h2><p className="review-path">{value.releaseTitle}</p><p className="acquisition-state">{t(states[value.state])}</p>
 <progress aria-label={value.movieTitle} value={value.progress ?? undefined} max={1}/><p>{value.progress === null ? t('Unknown') : Math.round(value.progress * 100) + '%'} · {bytes(value.sizeBytes)} · {value.downloadSpeed === null ? t('Unknown') : bytes(value.downloadSpeed) + '/s'}</p>
 <p className="hint">{t('Last check')}: {time(value.lastCheckedAt)} · {t('Last observation')}: {time(value.lastObservedAt)}</p>{value.pollState === 'running' && <p role="status">{t('Checking download…')}</p>}{value.pollState === 'queued' && <p className="hint">{t('Next check')}: {time(value.nextCheckAt)}</p>}
 {value.errorCode && <p>{t(reasons[value.errorCode] ?? 'Unable to read or update download tracking. Refresh before retrying.')}</p>}
 {value.handoff && <div className="handoff-evidence"><p>{t(value.handoff.state === 'awaiting-validation' ? 'Client completion recorded. Local file and movie identity still require validation.' : 'Transfer completed. Import evidence needs review.')}</p>{value.handoff.reasonCode && <p>{t(reasons[value.handoff.reasonCode] ?? 'File paths are unsafe or ambiguous. Manual review is required.')}</p>}{value.handoff.file && <p className="review-path">{value.handoff.file.relativePath} · {bytes(value.handoff.file.sizeBytes)}</p>}</div>}
 <div className="actions">{value.pollState === 'idle' && <button disabled={busy || loading} onClick={() => void check(value)}>{t('Check download')}</button>}{!acquisitionId && <a className="button secondary" href={'#/downloads/' + value.acquisitionId}>{t('View download history')}</a>}</div></article>)}</div>
 {!acquisitionId && list && <div className="actions"><button className="secondary" disabled={loading || busy || offset === 0} onClick={() => { setReadError(undefined); setOperationError(undefined); setOffset(Math.max(0, offset - 25)); }}>{t('Previous downloads')}</button><button className="secondary" disabled={loading || busy || offset + list.limit >= list.total} onClick={() => { setReadError(undefined); setOperationError(undefined); setOffset(offset + 25); }}>{t('Next downloads')}</button></div>}
 {detail && <section className="tracking-history"><h2>{t('Download history')}</h2><p className="hint">{t('Showing recent transitions')}: {detail.events.length} / {detail.eventTotal}</p><ol>{detail.events.map((event, index) => <li key={index}><time>{time(event.createdAt)}</time> · {t(states[event.state])}{event.progress !== null && <> · {Math.round(event.progress * 100)}%</>}{event.code && <p>{t(reasons[event.code] ?? 'Unable to read or update download tracking. Refresh before retrying.')}</p>}</li>)}</ol></section>}
 </section>;
}
