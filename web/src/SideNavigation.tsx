import { useUI } from './ui';
export function SideNavigation({ settingsPath = '/settings' }: { settingsPath?: string }) {
  const { t } = useUI();
  const active = window.location.pathname;
  const entries = [
    { href: '/preview', label: 'Library', icon: 'M3 3h18v18H3V3Zm0 6h18M8 3v6m8-6v6' },
    { href: '/search', label: 'Search movies', icon: 'm21 21-5-5M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0' },
    { href: settingsPath, label: settingsPath === '/setup' ? 'Setup' : 'Settings', icon: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Zm0-6v3m0 14v3M2 12h3m14 0h3' },
  ] as const;
  return <aside className="persistent-sidebar"><nav className="workspace-nav" aria-label={t('Workspace')}>
    {entries.map(entry => <a key={entry.href} className={'nav-item' + (active === entry.href ? ' selected' : '')} href={entry.href} aria-current={active === entry.href ? 'page' : undefined} title={t(entry.label)}><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><path d={entry.icon}/></svg><span className="navigation-label">{t(entry.label)}</span></a>)}
  </nav></aside>;
}
