import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { api, type Settings, type SetupState } from './api';
import { SetupWizard } from './SetupWizard';
import './style.css';

function App() {
  const [setup, setSetup] = useState<SetupState>();
  const [saved, setSaved] = useState<Settings>();
  const [error, setError] = useState('');
  const [path, setPath] = useState(window.location.pathname);
  const [editing, setEditing] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    api<SetupState>('/setup', { signal: controller.signal })
      .then(async state => {
        if (state.completed) setSaved(await api<Settings>('/settings', { signal: controller.signal }));
        setSetup(state);
      })
      .catch((failure: Error) => { if (failure.name !== 'AbortError') setError(failure.message); });
    return () => controller.abort();
  }, []);
  const afterSetup = (value: Settings) => {
    setEditing(false);
    setSaved(value); setSetup(current => current ? { ...current, completed: true } : current);
    window.history.replaceState(null, '', '/settings'); setPath('/settings');
  };
  const audioName = (code: string) => code === 'original' ? 'Original audio' : setup?.languages.find(language => language.code === code)?.name ?? code;
  return <main>
    <header><a className="brand" href="/"><span className="mark" aria-hidden="true">P</span><strong>Piratarr</strong></a><span className="local">Your library, locally.</span></header>
    <section className="intro">
      <p className="eyebrow">A place for your collection</p>
      <h1>{saved ? <>Your library.<br />Your preferences.</> : <>Your next movie<br />starts here.</>}</h1>
      <p className="description">One home for your movies, your preferences, and the files you keep. Everything stored on your own machine.</p>
    </section>
    <section className="panel">
      {error ? <><h2>Unable to read settings</h2><p role="alert">{error}</p><button onClick={() => window.location.reload()}>Try again</button></>
        : !setup ? <p role="status">Checking your installation…</p>
        : saved && editing ? <SetupWizard setup={setup} initial={saved} onSaved={afterSetup} onCancel={() => setEditing(false)} />
        : saved ? <>
          <span className="badge">Configured · revision {saved.revision}</span><h2>Your saved settings</h2>
          <dl className="summary"><dt>Download directory</dt><dd>{saved.directories.downloadDirectory}</dd><dt>Movie directory</dt><dd>{saved.directories.movieDirectory}</dd><dt>Accepted resolutions</dt><dd>{saved.profile.acceptedResolutions.join(', ')}</dd><dt>Preferred resolution</dt><dd>{saved.profile.preferredResolution}</dd><dt>Audio preference order</dt><dd>{saved.profile.audioPreferences.map(audioName).join(' → ')}</dd></dl>
          <button type="button" onClick={() => setEditing(true)}>Edit settings</button>
          <p className="note">Your choices are saved locally. Catalog, search, and downloads are not available yet.</p>
        </> : path === '/setup' ? <SetupWizard setup={setup} onSaved={afterSetup} /> : <>
          <span className="badge">Not configured yet</span><h2>Make this space yours</h2>
          <p>Start by choosing where your files live and how you prefer to watch.</p>
          <div className="steps"><span>01 · Local folders</span><span>02 · Picture quality</span><span>03 · Audio preferences</span></div>
          <a className="button" href="/setup">Start setup <span aria-hidden="true">↗</span></a>
          <p className="note">Choose your own preferences during setup. Catalog, search, and downloads are not available yet.</p>
        </>}
    </section>
    <footer>Stored locally. No Piratarr cloud account.</footer>
  </main>;
}

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>);
