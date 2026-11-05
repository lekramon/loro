import { SideNavigation } from './SideNavigation';
import { AppHeader } from './AppHeader';
import { useUI } from './ui';
export function About(){const {t}=useUI();return <main className="app-shell"><SideNavigation/><AppHeader title="About & credits"/><div className="workspace"><section className="panel"><h1>{t('About & credits')}</h1><h2>Louro</h2><p>{t('Movie metadata and artwork provided by TMDB.')}</p><a href="https://www.themoviedb.org" target="_blank" rel="noreferrer"><img className="tmdb-logo" src="/tmdb.svg" alt="TMDB" width="140"/></a><p lang="en-US">This product uses the TMDB API but is not endorsed or certified by TMDB.</p><a href="/settings" className="button">{t('Settings')}</a></section></div></main>}
