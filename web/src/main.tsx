import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { api, type Settings, type SetupState } from './api';
import { SetupWizard } from './SetupWizard';
import { UIProvider, useUI } from './ui';
import './style.css';
import { SideNavigation } from './SideNavigation';
import { AppHeader } from './AppHeader';
import { SourceSettings } from './SourceSettings';
import { DownloadClientSettings } from './DownloadClientSettings';
import { MetadataSettings } from './MetadataSettings';
import { About } from './About';
import { MockApp } from './MockApp';
import { settingsPath } from './SettingsNavigation';
import type { MessageKey } from './messages';

function PlannedSection({ title, description }: { title: MessageKey; description: MessageKey }) {
  const { t } = useUI();
  return <section className="planned-section" aria-labelledby="planned-heading"><h2 id="planned-heading">{t(title)}</h2><p>{t(description)}</p><p className="hint">{t('This section is planned and does not change local data yet.')}</p></section>;
}

function SettingsOverview({ saved, setup, audioName, onEdit }: { saved: Settings; setup: SetupState; audioName: (code: string) => string; onEdit: () => void }) {
  const { t } = useUI();
  return <>
    <span className="badge">{t('Configured · revision {revision}', { revision: saved.revision })}</span><h2>{t('Your saved settings')}</h2>
    <dl className="summary"><dt>{t('Download directory')}</dt><dd>{saved.directories.downloadDirectory}</dd><dt>{t('Movie directory')}</dt><dd>{saved.directories.movieDirectory}</dd><dt>{t('Accepted resolutions')}</dt><dd>{saved.profile.acceptedResolutions.join(', ')}</dd><dt>{t('Preferred resolution')}</dt><dd>{saved.profile.preferredResolution}</dd><dt>{t('Release formats')}</dt><dd>{(saved.profile.acceptedFormats ?? []).join(', ')}</dd><dt>{t('File containers')}</dt><dd>{(saved.profile.acceptedContainers ?? []).join(', ')}</dd><dt>{t('Audio preference order')}</dt><dd>{saved.profile.audioPreferences.map(audioName).join(' → ')}</dd></dl>
    <button type="button" onClick={onEdit}>{t('Edit settings')}</button>
    <p className="note">{t('Your choices are saved locally. Search movies from the sidebar; the library preview uses mock data.')}</p>
  </>;
}

function App() {
  const { t, errorText, languageName } = useUI();
  const [setup, setSetup] = useState<SetupState>(); const [saved, setSaved] = useState<Settings>(); const [error, setError] = useState('');
  const [path, setPath] = useState(window.location.pathname); const [editing, setEditing] = useState(false);
  useEffect(() => { const update = () => setPath(window.location.pathname); window.addEventListener('popstate', update); return () => window.removeEventListener('popstate', update); }, []);
  useEffect(() => { const controller = new AbortController(); api<SetupState>('/setup', { signal: controller.signal }).then(async state => { if (state.completed) setSaved(await api<Settings>('/settings', { signal: controller.signal })); setSetup(state); }).catch((failure: Error) => { if (failure.name !== 'AbortError') setError(failure.message); }); return () => controller.abort(); }, []);
  const afterSetup = (value: Settings) => { setEditing(false); setSaved(value); setSetup(current => current ? { ...current, completed: true } : current); window.history.replaceState(null, '', '/settings'); setPath('/settings'); };
  const audioName = (code: string) => languageName(code, setup?.languages.find(language => language.code === code)?.name);
  const section = settingsPath(path); const overview = path === '/settings' || path === '/settings/' || path === '/setup';
  const show = (value: string) => overview || section === value;
  return <main className="app-shell"><SideNavigation settingsPath={saved ? '/settings' : '/setup'} /><AppHeader title={saved ? 'Settings' : 'Setup'} /><div className="workspace">
    <section className="page-heading"><p className="eyebrow">{t('Your library, locally.')}</p><h1>{t(saved ? 'Settings' : 'Getting started')}</h1><p className="description">{t(saved ? 'Manage your folders and viewing preferences.' : 'A few preferences, then you are ready to build your library.')}</p></section>
    <section className="panel">
      {error ? <><h2>{t('Unable to read settings')}</h2><p role="alert">{errorText(error)}</p><button onClick={() => window.location.reload()}>{t('Try again')}</button></>
        : !setup ? <p role="status">{t('Checking your installation…')}</p>
        : saved && editing ? <SetupWizard setup={setup} initial={saved} onSaved={afterSetup} onCancel={() => setEditing(false)} />
        : saved ? <>
          {overview && <><span className="badge">{t('Configured · revision {revision}', { revision: saved.revision })}</span><h2>{t('Your saved settings')}</h2></>}
          {show('media-management') && <section aria-labelledby="media-management-heading"><h2 id="media-management-heading">{t('Media Management')}</h2><p className="hint">{t('Manage the folders used for downloads and movies. Existing files are preserved.')}</p><dl className="summary"><dt>{t('Download directory')}</dt><dd>{saved.directories.downloadDirectory}</dd><dt>{t('Movie directory')}</dt><dd>{saved.directories.movieDirectory}</dd></dl><button type="button" onClick={() => setEditing(true)}>{t('Edit settings')}</button></section>}
          {show('profiles') && <section aria-labelledby="profiles-heading"><h2 id="profiles-heading">{t('Profiles')}</h2><p className="hint">{t('Current acquisition preferences remain available here. Named profiles are planned for a later phase.')}</p><dl className="summary"><dt>{t('Accepted resolutions')}</dt><dd>{saved.profile.acceptedResolutions.join(', ')}</dd><dt>{t('Preferred resolution')}</dt><dd>{saved.profile.preferredResolution}</dd><dt>{t('Release formats')}</dt><dd>{(saved.profile.acceptedFormats ?? []).join(', ')}</dd><dt>{t('File containers')}</dt><dd>{(saved.profile.acceptedContainers ?? []).join(', ')}</dd><dt>{t('Audio preference order')}</dt><dd>{saved.profile.audioPreferences.map(audioName).join(' → ')}</dd></dl>{!overview && <button type="button" onClick={() => setEditing(true)}>{t('Edit settings')}</button>}</section>}
          {show('quality') && <PlannedSection title="Quality" description="Quality size definitions and richer quality policies are planned for a later phase." />}
          {show('custom-formats') && <PlannedSection title="Custom Formats" description="Custom release-format scoring is planned and no fake rules are saved here." />}
          {show('indexers') && <SourceSettings />}
          {show('download-clients') && <DownloadClientSettings />}
          {show('import-lists') && <PlannedSection title="Import Lists" description="Letterboxd and IMDb list import is planned. No external list is contacted by opening this section." />}
          {show('subtitles') && <PlannedSection title="Subtitles" description="Native subtitle management is planned. No subtitle provider request is made here." />}
          {show('general') && <MetadataSettings />}
          {show('system') && <section aria-labelledby="system-heading"><h2 id="system-heading">{t('System')}</h2><p>{t('Lôro is running as a local application. Readiness, configuration summaries and communications stay on this machine.')}</p><p className="hint">{t('Open Logs from the bottom-left control to inspect sanitized application communication.')}</p></section>}
        </> : path === '/setup' ? <SetupWizard setup={setup} onSaved={afterSetup} /> : <><span className="badge">{t('Not configured yet')}</span><h2>{t('Make this space yours')}</h2><p>{t('Start by choosing where your files live and how you prefer to watch.')}</p><div className="steps"><span>{t('01 · Local folders')}</span><span>{t('02 · Picture quality')}</span><span>{t('03 · Audio preferences')}</span></div><a className="button" href="/setup">{t('Start setup')} <span aria-hidden="true">↗</span></a><p className="note">{t('Choose your preferences during setup. Movie metadata search is available from the sidebar.')}</p></>}
    </section><footer>{t('Stored locally. No Lôro cloud account.')} · <a href="/about">{t('About & credits')}</a></footer>
  </div></main>;
}

function SearchRedirect() { useEffect(() => { window.location.replace('/preview#/search'); }, []); return null; }
createRoot(document.getElementById('root')!).render(<StrictMode><UIProvider>{window.location.pathname === '/preview' ? <MockApp /> : window.location.pathname === '/about' ? <About /> : window.location.pathname === '/search' ? <SearchRedirect /> : <App />}</UIProvider></StrictMode>);
