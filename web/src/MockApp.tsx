import { useEffect, useRef, useState } from 'react';
import { demoMovies, persistDemoState, readDemoState, type DemoMovie, type DemoViewer } from './mockData';
import { InterfacePreferences, useUI } from './ui';
import './mock.css';

const paths: Record<string,string> = {
 menu:'M4 6h16M4 12h16M4 18h16', search:'m21 21-5-5M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0',
 user:'M20 21v-2a7 7 0 0 0-14 0v2M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0', home:'m3 10 9-7 9 7v11h-6v-7H9v7H3V10Z',
 heart:'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z',
 movies:'M3 3h18v18H3V3Zm0 6h18M8 3v6m8-6v6', download:'M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5',
 check:'m4 12 5 5L20 6', back:'m14 5-7 7 7 7', close:'m5 5 14 14M5 19 19 5', play:'m8 4 13 8-13 8V4Z', pause:'M8 4v16M16 4v16', settings:'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Zm0-6v3m0 14v3M2 12h3m14 0h3M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2', plus:'M12 4v16M4 12h16',
};
function Icon({ name }: {name:string}) {return <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={paths[name]??paths.movies}/></svg>}
function Brand(){return <span className="demo-brand"><span className="demo-brand-mark">P</span>Piratarr</span>}
function routeNow(){return window.location.hash.slice(1)||'/profiles'}
const normalize=(s:string)=>s.normalize('NFD').replace(/\p{Diacritic}/gu,'').toLowerCase();
function go(path:string){window.location.hash=path}
type QueueItem={id:string;progress:number;paused:boolean};

export function MockApp(){
 const {t,locale}=useUI();
 const [state,setState]=useState(readDemoState);
 const [route,setRoute]=useState(routeNow);
 const [drawer,setDrawer]=useState(false);
 const [query,setQuery]=useState('');
 const [genre,setGenre]=useState('all');
 const [sort,setSort]=useState('title');
 const [notice,setNotice]=useState('');
 const [queue,setQueue]=useState<QueueItem[]>([{id:'orbit',progress:64,paused:false},{id:'wild-north',progress:28,paused:true},{id:'afterlight',progress:100,paused:false}]);
 const active=state.viewers.find(v=>v.id===state.activeId);
 useEffect(()=>{const changed=()=>{setRoute(routeNow());setDrawer(false);setNotice('');window.scrollTo(0,0)};window.addEventListener('hashchange',changed);return()=>window.removeEventListener('hashchange',changed)},[]);
 useEffect(()=>persistDemoState(state),[state]);
 const updateViewer=(patch:Partial<DemoViewer>)=>setState(current=>({...current,viewers:current.viewers.map(v=>v.id===current.activeId?{...v,...patch}:v)}));
 const toggle=(field:'favorites'|'watched',id:string)=>{if(!active)return;updateViewer({[field]:active[field].includes(id)?active[field].filter(x=>x!==id):[...active[field],id]})};
 const selectViewer=(id:string)=>{setState(current=>({...current,activeId:id}));go('/home')};
 const selectedMovie=demoMovies.find(m=>m.id===route.split('/')[2]);
 const isProfiles=!active||route==='/profiles'||route==='/profiles/new';
 const favoriteMovies=demoMovies.filter(m=>active?.favorites.includes(m.id));
 const filtered=demoMovies.filter(m=>(genre==='all'||m.genre===genre)&&normalize(m.title).includes(normalize(query))).sort((a,b)=>sort==='year'?b.year-a.year:sort==='rating'?Number(b.rating)-Number(a.rating):a.title.localeCompare(b.title,locale));
 const visibleMovies=route==='/favorites'?favoriteMovies:filtered;
 const portrait=(movie:DemoMovie,wide=false)=><a className={'media-card'+(wide?' landscape':'')} href={'#/movie/'+movie.id} key={movie.id} aria-label={movie.title+' ('+movie.year+')'}><div className="media-art"><img src={'/mock-art/'+movie.id+(wide?'-wide':'')+'.svg'} alt="" loading="lazy"/>{active?.watched.includes(movie.id)&&<span className="seen-badge" aria-label={t('Watched')}><Icon name="check"/></span>}<span className="card-hover"><Icon name="play"/></span>{wide&&movie.progress&&!active?.watched.includes(movie.id)&&<span className="media-progress"><span style={{width:movie.progress+'%'}}/></span>}</div><span className="media-title">{movie.title}</span><span className="media-year">{movie.year}</span></a>;
 const row=(heading:Parameters<typeof t>[0],movies:DemoMovie[],wide=false)=>movies.length>0?<section className="media-section"><h2><a href="#/movies">{t(heading)} <span aria-hidden="true">›</span></a></h2><div className={'media-row'+(wide?' wide-row':'')}>{movies.map(m=>portrait(m,wide))}</div></section>:null;
 return <main className="demo-app">
  <header className="demo-header">
   <div className="demo-header-start">{!isProfiles&&<button className="icon-button" aria-label={t('Open navigation')} onClick={()=>setDrawer(true)}><Icon name="menu"/></button>}<a href={isProfiles?'#/profiles':'#/home'} aria-label="Piratarr"><Brand/></a></div>
   {!isProfiles&&<nav className="demo-tabs" aria-label={t('Main navigation')}><a href="#/home" aria-current={route==='/home'?'page':undefined}>{t('Home')}</a><a href="#/favorites" aria-current={route==='/favorites'?'page':undefined}>{t('Favorites')}</a></nav>}
   <div className="demo-header-end">{!isProfiles&&<><a className="icon-button" href="#/search" aria-label={t('Search')}><Icon name="search"/></a><a className="viewer-chip" href="#/profile" aria-label={t('Viewer profile')+': '+active?.name}><span style={{background:active?.color}}>{active?.name.slice(0,1).toUpperCase()}</span><span className="viewer-name">{active?.name}</span></a></>}<InterfacePreferences/></div>
  </header>
  <div className="demo-indicator"><span>{t('Mock data · Local prototype')}</span><a href="/settings">{t('Real settings')}</a></div>
  {drawer&&<NavigationDrawer close={()=>setDrawer(false)}/>}
  {notice&&<p className="demo-notice" role="status">{notice}</p>}
  {isProfiles?<section className="profile-screen">
   {route==='/profiles/new'?<ProfileForm onCancel={()=>go('/profiles')} onSave={name=>{const id='viewer-'+crypto.randomUUID();setState(current=>({...current,viewers:[...current.viewers,{id,name,color:'#7951b4',favorites:[],watched:[]}],activeId:id}));go('/home')}}/>:<>
    <h1>{t('Who is watching?')}</h1><p>{t('Choose a viewer to explore the prototype.')}</p><div className="profile-grid">{state.viewers.map(v=><button className="profile-choice" key={v.id} onClick={()=>selectViewer(v.id)}><span className="profile-avatar" style={{background:v.color}}><Icon name="user"/></span><span>{v.name}</span></button>)}{state.viewers.length<12&&<a className="profile-choice add-profile" href="#/profiles/new"><span className="profile-avatar"><Icon name="plus"/></span><span>{t('Add profile')}</span></a>}</div><p className="profile-disclaimer">{t('Demo profiles personalize this browser only. They are not accounts or access controls.')}</p><a className="demo-text-link" href="/settings">{t('Real settings')}</a>
   </>}
  </section>:route==='/home'?<div className="demo-content">
   <section className="media-section"><h1>{t('My media')}</h1><div className="library-tiles"><a href="#/movies" className="library-tile"><img src="/mock-art/wild-north-wide.svg" alt=""/><strong>{t('Movies')}</strong></a><a href="#/favorites" className="library-tile"><img src="/mock-art/deep-blue-wide.svg" alt=""/><strong>{t('Favorites')}</strong></a><a href="#/downloads" className="library-tile download-tile"><Icon name="download"/><strong>{t('Downloads')}</strong></a></div></section>
   {row('Continue watching',demoMovies.filter(m=>m.progress&&!active.watched.includes(m.id)),true)}
   {row('Latest movies',demoMovies)}
   {favoriteMovies.length>0&&row('Your favorites',favoriteMovies)}
  </div>:route==='/movies'||route==='/search'||route==='/favorites'?<div className="demo-content">
   <h1>{t(route==='/search'?'Search':route==='/favorites'?'Favorites':'Movies')}</h1>
   {route!=='/favorites'&&<div className="library-toolbar"><div className="movie-search"><Icon name="search"/><label className="visually-hidden" htmlFor="demo-search">{t('Search movies')}</label><input id="demo-search" type="search" value={query} onChange={e=>setQuery(e.target.value)} placeholder={t('Search movies')}/></div><label>{t('Genre')}<select aria-label={t('Genre')} value={genre} onChange={e=>setGenre(e.target.value)}><option value="all">{t('All genres')}</option>{(['Science fiction','Drama','Adventure','Thriller'] as const).map(g=><option key={g} value={g}>{t(g)}</option>)}</select></label><label>{t('Sort by')}<select aria-label={t('Sort by')} value={sort} onChange={e=>setSort(e.target.value)}><option value="title">{t('Title')}</option><option value="year">{t('Year')}</option><option value="rating">{t('Rating')}</option></select></label></div>}
   <p className="result-count">{t(visibleMovies.length===1?'{count} movie':'{count} movies',{count:visibleMovies.length})}</p>
   {visibleMovies.length?<div className="poster-grid">{visibleMovies.map(m=>portrait(m))}</div>:<div className="demo-empty"><Icon name={route==='/favorites'?'heart':'search'}/><h2>{t(route==='/favorites'?'No favorites yet':'No movies found')}</h2><p>{t(route==='/favorites'?'Add favorites from a movie’s details.':'Try a different title or genre.')}</p><a href="#/movies" className="button">{t('Browse movies')}</a></div>}
  </div>:route.startsWith('/movie/')&&selectedMovie?<>
   <div className="movie-backdrop"><img src={'/mock-art/'+selectedMovie.id+'-wide.svg'} alt=""/></div><section className="movie-details"><img className="detail-poster" src={'/mock-art/'+selectedMovie.id+'.svg'} alt=""/><div className="detail-copy"><a href="#/movies" className="detail-back"><Icon name="back"/>{t('Movies')}</a><h1>{selectedMovie.title}</h1><div className="detail-meta"><span>{selectedMovie.year}</span><span>{selectedMovie.minutes} min</span><span className="quality-tag">{selectedMovie.resolution}</span><span className="quality-tag">{selectedMovie.format}</span><span>★ {selectedMovie.rating}</span></div><p className="detail-genre">{t(selectedMovie.genre)}</p><div className="detail-actions"><a className="button demo-primary" href={'#/player/'+selectedMovie.id}><Icon name="play"/>{t('Play')}</a><button className="round-action" aria-label={t('Favorite')} aria-pressed={active.favorites.includes(selectedMovie.id)} onClick={()=>toggle('favorites',selectedMovie.id)}><Icon name="heart"/></button><button className="round-action" aria-label={t('Watched')} aria-pressed={active.watched.includes(selectedMovie.id)} onClick={()=>toggle('watched',selectedMovie.id)}><Icon name="check"/></button><button className="round-action" aria-label={t('Add to mock downloads')} disabled={queue.some(q=>q.id===selectedMovie.id)} onClick={()=>{setQueue(q=>[...q,{id:selectedMovie.id,progress:0,paused:true}]);setNotice(t('Added to mock queue.'))}}><Icon name="download"/></button></div><p className="synopsis">{selectedMovie.synopsis[locale]}</p><dl className="media-info"><dt>{t('Video')}</dt><dd>HEVC · {selectedMovie.resolution} · {selectedMovie.format}</dd><dt>{t('Audio')}</dt><dd>{selectedMovie.audio}</dd><dt>{t('Subtitles')}</dt><dd>English · Português (Brasil)</dd></dl></div></section>
   <div className="demo-content related-content">{row('More like this',demoMovies.filter(m=>m.genre===selectedMovie.genre&&m.id!==selectedMovie.id))}</div>
  </>:route==='/downloads'?<div className="demo-content"><h1>{t('Downloads')}</h1><p className="demo-muted">{t('Simulated transfers. No files are downloaded.')}</p><div className="download-list">{queue.map(q=>{const movie=demoMovies.find(m=>m.id===q.id)!;return <article className="download-card" key={q.id}><a href={'#/movie/'+q.id}><img src={'/mock-art/'+q.id+'.svg'} alt=""/></a><div><h2><a href={'#/movie/'+q.id}>{movie.title}</a></h2><p>{movie.resolution} · {movie.format}</p><progress value={q.progress} max="100" aria-label={movie.title}/><p>{q.progress}% · {t(q.progress===100?'Completed':q.paused?'Paused':'Downloading')}</p></div>{q.progress<100?<button className="secondary" onClick={()=>setQueue(current=>current.map(item=>item.id===q.id?{...item,paused:!item.paused}:item))}>{t(q.paused?'Resume':'Pause')}</button>:<a className="demo-text-link" href={'#/movie/'+q.id}>{t('View movie')}</a>}</article>})}</div></div>
  :route==='/profile'?<div className="demo-content viewer-settings"><h1>{t('Viewer profile')}</h1><ProfileForm key={active.id} initial={active.name} onCancel={()=>go('/home')} onSave={name=>{updateViewer({name});setNotice(t('Profile saved in this browser.'))}}/><a className="button secondary" href="#/profiles">{t('Switch profile')}</a><p className="profile-disclaimer">{t('Demo profiles personalize this browser only. They are not accounts or access controls.')}</p></div>
  :route.startsWith('/player/')&&selectedMovie?<DemoPlayer key={selectedMovie.id} movie={selectedMovie}/>
  :<div className="demo-content demo-empty"><h1>{t('Page not found')}</h1><a href="#/home" className="button">{t('Home')}</a></div>}
 </main>;
}

function NavigationDrawer({close}:{close:()=>void}){
 const {t}=useUI();const ref=useRef<HTMLDialogElement>(null);
 useEffect(()=>{ref.current?.showModal()},[]);
 return <dialog className="demo-drawer" ref={ref} onCancel={close} onClick={e=>{if(e.target===e.currentTarget)close()}} aria-label={t('Main navigation')}><div className="drawer-heading"><Brand/><button className="icon-button" onClick={close} aria-label={t('Close navigation')}><Icon name="close"/></button></div><nav>{(['home','movies','search','favorites','downloads','profile','profiles'] as const).map((route,i)=><a key={route} href={'#/'+route} onClick={close}><Icon name={['home','movies','search','heart','download','user','user'][i]}/>{t((['Home','Movies','Search','Favorites','Downloads','Viewer profile','Switch profile'] as const)[i])}</a>)}<a href="/settings"><Icon name="settings"/>{t('Real settings')}</a></nav><p>{t('Mock data · Local prototype')}</p></dialog>
}
function ProfileForm({initial='',onSave,onCancel}:{initial?:string;onSave:(name:string)=>void;onCancel:()=>void}){
 const {t}=useUI();const [name,setName]=useState(initial);return <form className="profile-form" onSubmit={e=>{e.preventDefault();if(name.trim())onSave(name.trim())}}><h2>{t(initial?'Edit profile':'Add profile')}</h2><label htmlFor="viewer-name">{t('Display name')}</label><input id="viewer-name" value={name} onChange={e=>setName(e.target.value)} maxLength={30} required autoComplete="off"/><div className="actions"><button type="submit" disabled={!name.trim()}>{t('Save profile')}</button><button type="button" className="secondary" onClick={onCancel}>{t('Cancel')}</button></div></form>
}
function DemoPlayer({movie}:{movie:DemoMovie}){
 const {t}=useUI();const [playing,setPlaying]=useState(false);const [position,setPosition]=useState(movie.progress??0);
 return <section className="demo-player"><img src={'/mock-art/'+movie.id+'-wide.svg'} alt=""/><div className="player-heading"><a href={'#/movie/'+movie.id} className="icon-button" aria-label={t('Back')}><Icon name="back"/></a><h1>{movie.title}</h1></div><div className="player-simulation"><Icon name="play"/><p>{t('Playback simulation — no video is being streamed.')}</p></div><div className="player-controls"><button className="icon-button" onClick={()=>setPlaying(v=>!v)} aria-label={t(playing?'Pause':'Play')}><Icon name={playing?'pause':'play'}/></button><label className="visually-hidden" htmlFor="demo-position">{t('Playback position')}</label><input id="demo-position" type="range" min="0" max="100" value={position} onChange={e=>setPosition(Number(e.target.value))}/><span>{Math.round(movie.minutes*position/100)} / {movie.minutes} min</span><span role="status">{t(playing?'Simulating playback':'Paused')}</span></div></section>
}
