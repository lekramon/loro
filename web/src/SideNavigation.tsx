import { useEffect, useState } from 'react';
import { useUI } from './ui';
import { CommunicationConsole } from './CommunicationConsole';
import { SettingsNavigation } from './SettingsNavigation';

export function SideNavigation({ settingsPath = '/settings' }: { settingsPath?: string }) {
  const { t } = useUI();
  const [hash, setHash] = useState(window.location.hash);
  useEffect(() => { const update = () => setHash(window.location.hash); window.addEventListener('hashchange', update); return () => window.removeEventListener('hashchange', update); }, []);
  const icons = ['m3 10 9-7 9 7v11h-6v-7H9v7H3V10Z', 'M3 3h18v18H3V3Zm0 6h18M8 3v6m8-6v6', 'M3 3h18v18H3V3Zm0 6h18', 'M4 3h11l5 5v13H4V3Zm10 0v6h6', 'm21 21-5-5M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0', 'M4 4h16v16H4V4Zm4 4h8M8 12h8m-8 4h5', 'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z', 'M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5', 'M20 21v-2a7 7 0 0 0-14 0v2M16 7a4 4 0 1 1-8 0'];
  const library = window.location.pathname === '/preview' || window.location.pathname === '/search';
  const routes = ['home', 'movies', 'series', 'anime', 'files', 'search', 'releases', 'favorites', 'downloads', 'profiles'] as const;
  const labels = ['Home', 'Movies', 'Series', 'Anime', 'Files', 'Search', 'Releases', 'Favorites', 'Downloads', 'Switch profile'] as const;
  return <aside className="persistent-sidebar"><nav className="workspace-nav" aria-label={t('Workspace')}>
    <a className={'nav-item' + (library ? ' selected' : '')} href="/preview#/home" aria-current={library ? 'page' : undefined} title={t('Library')}><img className="app-brand-icon" src="/brand/parrot.png" alt="" width="24" height="24" /><span className="navigation-label">{t('Library')}</span></a>
    {library && <nav className="library-navigation" aria-label={t('Library navigation')}>{routes.map((route, index) => <a key={route} href={'/preview#/' + route} aria-current={(hash.split('?')[0] === '#/' + route || route === 'downloads' && hash.startsWith('#/downloads/') || !hash && route === 'profiles') ? 'page' : undefined} title={t(labels[index])}><svg className="library-nav-icon" width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={icons[index % icons.length]} /></svg><span className="navigation-label">{t(labels[index])}</span></a>)}</nav>}
    <a className={'nav-item' + (!library ? ' selected' : '')} href={settingsPath} aria-current={!library ? 'page' : undefined} title={t(settingsPath === '/setup' ? 'Setup' : 'Settings')}><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true"><path d="M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Zm0-6v3m0 14v3M2 12h3m14 0h3" /></svg><span className="navigation-label">{t(settingsPath === '/setup' ? 'Setup' : 'Settings')}</span></a>
    {!library && settingsPath !== '/setup' && <SettingsNavigation path={window.location.pathname} />}
  </nav><CommunicationConsole /></aside>;
}
