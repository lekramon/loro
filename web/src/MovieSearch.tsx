import { useState } from 'react';
import { api } from './api';
import { metadataError } from './metadataErrors';
import { SideNavigation } from './SideNavigation';
import { InterfacePreferences, useUI } from './ui';
type Movie = { tmdbId: number; title: string; year: number | null; overview: string; posterUrl: string };
type Results = { page: number; totalPages: number; totalResults: number; results: Movie[]; cache: { hit: boolean; stale: boolean } };
export function MovieSearch() {
  const { t, locale } = useUI();
  const [query, setQuery] = useState(''); const [year, setYear] = useState('');
  const [result, setResult] = useState<Results>(); const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState<{query: string; year: string; language: string}>();
  async function search(page = 1, paginate = false) {
    if (loading) return;
    const parameters = !paginate ? { query: query.trim(), year, language: locale } : searched;
    if (!parameters?.query) return;
    setLoading(true); setError(''); setResult(undefined); setSearched(parameters);
    const values = new URLSearchParams({ query: parameters.query, language: parameters.language, page: String(page) });
    if (parameters.year) values.set('year', parameters.year);
    try { setResult(await api<Results>('/metadata/search?' + values)); }
    catch (failure) { setError(metadataError(failure, t)); }
    finally { setLoading(false); }
  }
  return <main className="app-shell"><SideNavigation /><header className="topbar"><span>{t('Search movies')}</span><InterfacePreferences /></header><div className="workspace live-search">
    <h1>{t('Search movies on TMDB')}</h1><p>{t('Real movie metadata. This search is separate from the mock library.')}</p>
    <form onSubmit={event => { event.preventDefault(); void search(); }}><fieldset disabled={loading}><label htmlFor="live-query">{t('Movie title')}</label><input id="live-query" type="search" value={query} onChange={event => setQuery(event.target.value)} maxLength={200} required /><label htmlFor="live-year">{t('Year (optional)')}</label><input id="live-year" type="number" min="1870" max="2100" value={year} onChange={event => setYear(event.target.value)} /><button disabled={!query.trim()}>{t(loading ? 'Searching…' : 'Search')}</button></fieldset></form>
    {error && <p role="alert">{error} <a href="/settings">{t('Settings')}</a></p>}
    {loading && <p role="status">{t('Searching…')}</p>}
    {result && <><p role="status">{t('{count} movies', { count: result.totalResults })}</p>{result.cache.stale && <p>{t('Showing cached results because TMDB is unavailable.')}</p>}<div className="live-results">{result.results.map(movie => <article key={movie.tmdbId}>{movie.posterUrl ? <img src={movie.posterUrl} alt="" loading="lazy" /> : <div className="poster-placeholder" aria-hidden="true">◇</div>}<div><h2>{movie.title}</h2><p>{movie.year ?? t('Unknown year')}</p><p>{movie.overview}</p></div></article>)}</div>{result.totalResults === 0 && <p>{t('No movies found')}</p>}<div className="actions"><button className="secondary" disabled={loading || result.page <= 1} onClick={() => void search(result.page - 1, true)}>{t('Previous page')}</button><span>{result.page} / {Math.max(1, Math.min(500, result.totalPages))}</span><button className="secondary" disabled={loading || result.page >= Math.min(500, result.totalPages)} onClick={() => void search(result.page + 1, true)}>{t('Next page')}</button></div></>}
    <footer><a href="/about">{t('About & credits')}</a><p>{t('Movie metadata and artwork provided by TMDB.')}</p></footer>
  </div></main>;
}
