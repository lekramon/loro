import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './style.css';
type Setup = { completed: boolean };
function App() {
  const [setup,setSetup]=useState<Setup>(); const [error,setError]=useState('');
  useEffect(()=>{ const controller=new AbortController();
    fetch('/api/v1/setup',{signal:controller.signal}).then(async response=>{
      if(!response.ok) throw new Error('Local settings are unavailable.'); return response.json() as Promise<Setup>;
    }).then(setSetup).catch((failure:Error)=>{if(failure.name!=='AbortError')setError(failure.message);});
    return ()=>controller.abort();
  },[]);
  const starting=window.location.pathname==='/setup';
  return <main><header><span className="mark" aria-hidden="true">P</span><strong>Piratarr</strong><span className="local">Your library, locally.</span></header>
    <section className="intro"><p className="eyebrow">A place for your collection</p><h1>Your next movie<br/>starts here.</h1><p className="description">One home for your movies, your preferences, and the files you keep. Everything stored on your own machine.</p></section>
    <section className="panel" aria-live="polite">{error?<><h2>Unable to read setup status</h2><p role="alert">{error}</p><button onClick={()=>window.location.reload()}>Try again</button></>:!setup?<p>Checking your installation…</p>:<><span className="badge">{setup.completed?'Configured':'Not configured yet'}</span><h2>{starting?'Set up your library':'Make this space yours'}</h2><p>{starting?'The setup wizard is coming next. Your directories, resolutions, and audio preferences will be chosen here.':'Start by choosing where your files live and how you prefer to watch.'}</p><div className="steps"><span>01 · Local folders</span><span>02 · Picture quality</span><span>03 · Audio preferences</span></div>{!starting&&<a className="button" href="/setup">Explore setup ↗</a>}<p className="note">Foundation preview. Setup saving, catalog, search, and downloads are not available in this build.</p></>}</section><footer>Stored locally. No Piratarr cloud account.</footer></main>;
}
createRoot(document.getElementById('root')!).render(<StrictMode><App/></StrictMode>);
