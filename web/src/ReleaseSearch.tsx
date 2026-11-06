import { useEffect, useRef, useState } from 'react';
import { api, ApiError } from './api';
import { useUI } from './ui';
import { AcquisitionHistory, ReleaseSelection } from './Acquisitions';

type Movie = { id: number; title: string; year: number | null };
type Catalog = { items: Movie[]; total: number; offset: number; limit: number };
type Source = { name: string; configured: boolean; capabilities: { categories: { id: number; name: string }[] } | null };
type Check = { dimension: 'resolution' | 'format' | 'container' | 'audio'; status: 'match' | 'mismatch' | 'unknown'; accepted: string[] };
export type Candidate = { observationId: string; id: string; title: string; resolution: string | null; format: string | null; container: string | null; audioLanguages: string[]; sizeBytes: number | null; seeders: number | null; publishedAt: string | null; preferredResolution: boolean; status: Check['status']; audioPreferenceRank: number | null; checks: Check[] };
type Page = { items: Candidate[]; sourceName: string; sourceRevision: number; profileRevision: number; limit: number; offset: number; nextOffset: number | null; total: number | null; hasMore: boolean };

export function ReleaseSearch() {
  const { t, locale } = useUI();
  const pendingSearch = useRef<AbortController | null>(null);
  useEffect(() => () => { pendingSearch.current?.abort(); pendingSearch.current = null; }, []);
  const [catalog, setCatalog] = useState<Catalog>(); const [query, setQuery] = useState(''); const [filter, setFilter] = useState('');
  const [movie, setMovie] = useState<Movie>(); const [source, setSource] = useState<Source>(); const [category, setCategory] = useState('');
  const [selected, setSelected] = useState<Candidate>(); const [historyVersion, setHistoryVersion] = useState(0);
  const [results, setResults] = useState<Page>(); const [busy, setBusy] = useState(false); const [error, setError] = useState('');
  const statusLabel = { match: 'Accepted', mismatch: 'Outside profile', unknown: 'Unknown' } as const;
  const dimensionLabel = { resolution: 'Resolution', format: 'Release format', container: 'File container', audio: 'Audio' } as const;
  function failure(value: unknown) {
    if (value instanceof ApiError) {
      switch (value.code) {
        case 'source-unconfigured': return t('Configure a release source in Settings.');
        case 'source-capabilities-required': return t('Test the saved source in Settings before searching.');
        case 'source-search-unsupported': return t('This source does not advertise a supported movie search.');
        case 'source-authentication-failed': return t('The source rejected its credentials. Check the API key.');
        case 'source-timeout': return t('The source timed out. Check its address and connectivity.');
        case 'source-rate-limited': return t('The source is rate limiting requests. Wait before searching again.');
        case 'source-challenge-required': return t('The source requires browser verification. Check the source in Settings.');
        case 'source-invalid-response': return t('The source returned an invalid release feed.');
        case 'source-changed': case 'preferences-changed': return t('Source or preferences changed. Reload and search again.');
        case 'source-test-in-progress': return t('A source operation is running. Try again shortly.');
        case 'setup-required': return t('Complete setup before searching releases.');
        case 'validation-failed': return t('Reload the source and choose a current category.');
      }
    }
    return t('Unable to load release search. Retry or check Settings.');
  }
  async function loadCatalog(offset = 0, title = query.trim()) {
    setBusy(true); setError('');
    try { setCatalog(await api<Catalog>('/catalog/movies?' + new URLSearchParams({ limit: '25', offset: String(offset), q: title }))); setFilter(title); }
    catch (value) { setError(failure(value)); setCatalog(undefined); }
    finally { setBusy(false); }
  }
  async function loadSource() {

	setBusy(true); setError('');
    try { setSource(await api<Source>('/source')); setCategory(''); setResults(undefined); }
    catch (value) { setError(failure(value)); }
	finally { setBusy(false); }
  }
  useEffect(() => {
    let mounted = true;
    const id = new URLSearchParams(window.location.hash.split('?')[1]).get('movieId');
    void api<Catalog>('/catalog/movies?limit=25&offset=0').then(value => { if (mounted) setCatalog(value); }).catch(value => { if (mounted) setError(failure(value)); });
    void api<Source>('/source').then(value => { if (mounted) setSource(value); }).catch(value => { if (mounted) setError(failure(value)); });
    if (id && /^\d+$/.test(id)) void api<Movie>('/catalog/movies/' + id).then(value => { if (mounted) setMovie(value); }).catch(value => { if (mounted) setError(failure(value)); });
    return () => { mounted = false; };
  }, []);
  async function search(offset = 0) {
    if (!movie || busy) return;
    setBusy(true); setError(''); setResults(undefined);
    const params = new URLSearchParams({ movieId: String(movie.id), limit: '25', offset: String(offset) });
    if (category) params.set('category', category);
    const controller = new AbortController(); pendingSearch.current = controller;
    try { const page = await api<Page>('/releases?' + params, { signal: controller.signal }); if (!controller.signal.aborted) setResults(page); }
    catch (value) { if (!controller.signal.aborted) setError(failure(value)); }
    finally { if (pendingSearch.current === controller) { pendingSearch.current = null; setBusy(false); } }
  }
  function valueFor(candidate: Candidate, dimension: Check['dimension']) {
    if (dimension === 'audio') return candidate.audioLanguages.length ? candidate.audioLanguages.join(', ') : t('Unknown');
    return candidate[dimension] ?? t('Unknown');
  }
  return <section className="demo-content release-search">
    <h1>{t('Releases')}</h1><p className="demo-muted">{t('Search your source for a registered movie. No downloads are started.')}</p>
    {error && <p role="alert">{error} <a href="/settings">{t('Settings')}</a></p>}
    <div className="release-workspace">
      <section className="registered-movies" aria-label={t('Registered movies')}>
        <h2>{t('Registered movies')}</h2><form onSubmit={event => { event.preventDefault(); void loadCatalog(); }}><fieldset disabled={busy}><label htmlFor="catalog-filter">{t('Filter registered movies')}</label><input id="catalog-filter" type="search" maxLength={200} value={query} onChange={event => setQuery(event.target.value)} /><button className="secondary">{t('Filter')}</button></fieldset></form>
        {!catalog ? <p role="status">{t('Loading…')}</p> : <><ul className="registered-list">{catalog.items.map(item => <li key={item.id}><button className={'secondary' + (movie?.id === item.id ? ' chosen' : '')} disabled={busy} aria-pressed={movie?.id === item.id} onClick={() => { setMovie(item); setResults(undefined); setError(''); }}>{item.title}<small>{item.year ?? t('Unknown year')}</small></button></li>)}</ul>{!catalog.items.length && <p>{t('No registered movies. Find a movie on TMDB and add it first.')}</p>}<a href="#/search">{t('Search movies on TMDB')}</a><div className="actions"><button className="secondary" disabled={busy || catalog.offset === 0} onClick={() => void loadCatalog(Math.max(0, catalog.offset - catalog.limit), filter)}>{t('Previous page')}</button><button className="secondary" disabled={busy || catalog.offset + catalog.limit >= catalog.total} onClick={() => void loadCatalog(catalog.offset + catalog.limit, filter)}>{t('Next page')}</button></div></>}
      </section>
      <section className="release-results" aria-label={t('Release results')}>
        {movie ? <><h2>{movie.title} <span className="demo-muted">{movie.year ?? t('Unknown year')}</span></h2><form onSubmit={event => { event.preventDefault(); void search(); }}><fieldset disabled={busy}><p>{t('Release source')}: {source?.name || t('No source configured')}</p><label htmlFor="release-category">{t('Category')}</label><select id="release-category" value={category} onChange={event => { setCategory(event.target.value); setResults(undefined); }}><option value="">{t('All categories')}</option>{source?.capabilities?.categories.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select><div className="actions"><button>{t(busy ? 'Searching…' : 'Search releases')}</button><button type="button" className="secondary" onClick={() => void loadSource()}>{t('Reload source')}</button></div></fieldset></form></> : <p>{t('Choose a registered movie to search for releases.')}</p>}
        {busy && <p role="status">{t('Loading…')}</p>}
        {pendingSearch.current && <button type="button" className="secondary" onClick={() => { pendingSearch.current?.abort(); pendingSearch.current = null; setBusy(false); }}>{t('Cancel search')}</button>}
        {results && <><p role="status" className="demo-muted">{results.sourceName} · {results.items.length} {t('results on this page')}</p><p className="hint">{t('Source claims do not verify movie identity. Unknown attributes stay unknown.')}</p>{!results.items.length && <p>{t('No releases found.')}</p>}
          <div className="release-list">{results.items.map(candidate => <article className="release-card" key={candidate.id}><div className="release-heading"><h3>{candidate.title}</h3><span className={'release-status ' + candidate.status}>{t(statusLabel[candidate.status])}</span></div><p className="demo-muted">{t('Size')}: {candidate.sizeBytes === null ? t('Unknown') : `${(candidate.sizeBytes / 1073741824).toLocaleString(locale, { maximumFractionDigits: 2 })} GiB`} · {t('Seeders')}: {candidate.seeders ?? t('Unknown')}{candidate.publishedAt && <> · {new Date(candidate.publishedAt).toLocaleDateString(locale)}</>}</p>{candidate.preferredResolution && <p className="release-preferred">{t('Preferred resolution')}</p>}<dl className="release-checks">{candidate.checks.map(check => <div key={check.dimension}><dt>{t(dimensionLabel[check.dimension])}</dt><dd><strong>{valueFor(candidate, check.dimension)}</strong><span className={'release-status ' + check.status}>{t(statusLabel[check.status])}</span><small>{t('Profile')}: {check.accepted.map(value => value === 'original' ? t('Original audio') : value).join(', ')}</small></dd></div>)}</dl>{candidate.audioPreferenceRank !== null && <p className="hint">{t('Audio preference rank')}: {candidate.audioPreferenceRank}</p>}<button className="secondary" disabled={busy || !candidate.observationId} onClick={() => setSelected(candidate)}>{t('Select release')}</button></article>)}</div>
          <div className="actions"><button className="secondary" disabled={busy || results.offset === 0} onClick={() => void search(Math.max(0, results.offset - results.limit))}>{t('Previous releases')}</button><span>{results.items.length ? results.offset + 1 : results.offset}–{results.offset + results.items.length}{results.total !== null ? ` / ${results.total}` : ''}</span><button className="secondary" disabled={busy || results.nextOffset === null} onClick={() => void search(results.nextOffset!)}>{t('Next releases')}</button></div>
        </>}
      </section>
    </div>
    {movie && <AcquisitionHistory key={movie.id} movieId={movie.id} version={historyVersion} />}
    {movie && selected && <ReleaseSelection candidate={selected} movieTitle={movie.title} onClose={() => setSelected(undefined)} onSaved={() => setHistoryVersion(value => value + 1)} />}
  </section>;
}
