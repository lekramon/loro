import { useEffect, useState } from 'react';
import { demoMovies, persistDemoState, readDemoState, type DemoMovie, type DemoViewer } from './mockData';
import { SideNavigation } from './SideNavigation';
import { AppHeader } from './AppHeader';
import { LibraryFiles } from './LibraryFiles';
import { MovieSearch } from './MovieSearch';
import { ReleaseSearch } from './ReleaseSearch';
import { DownloadsPage } from './Downloads';
import { ProfileForm, ProfileAvatar, Pencil } from './ProfileForm';
import { useUI } from './ui';
import './mock.css';

const paths: Record<string,string> = {
 menu:'M4 6h16M4 12h16M4 18h16', search:'m21 21-5-5M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0',
 user:'M20 21v-2a7 7 0 0 0-14 0v2M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0', home:'m3 10 9-7 9 7v11h-6v-7H9v7H3V10Z',
 heart:'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z',
 movies:'M3 3h18v18H3V3Zm0 6h18M8 3v6m8-6v6', download:'M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5',
 check:'m4 12 5 5L20 6', back:'m14 5-7 7 7 7', close:'m5 5 14 14M5 19 19 5', play:'m8 4 13 8-13 8V4Z', pause:'M8 4v16M16 4v16', settings:'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Zm0-6v3m0 14v3M2 12h3m14 0h3M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2', plus:'M12 4v16M4 12h16',
};
function Icon({ name }: {name:string}) {return <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={paths[name]??paths.movies}/></svg>}
function routeNow(){return window.location.hash.slice(1)||'/profiles'}
const normalize=(s:string)=>s.normalize('NFD').replace(/\p{Diacritic}/gu,'').toLowerCase();
function go(path:string){window.location.hash=path}


export function MockApp(){
 const {t,locale}=useUI();
 const [state,setState]=useState(readDemoState);
 const [route,setRoute]=useState(routeNow);
 const [managing,setManaging]=useState(false);
 const [storageFailed,setStorageFailed]=useState(false);
 const [query,setQuery]=useState('');
 const [genre,setGenre]=useState('all');
 const [sort,setSort]=useState('title');
 const [notice,setNotice]=useState('');
 const active=state.viewers.find(v=>v.id===state.activeId);
 useEffect(()=>{const changed=()=>{setRoute(routeNow());setNotice('');window.scrollTo(0,0)};window.addEventListener('hashchange',changed);return()=>window.removeEventListener('hashchange',changed)},[]);
 useEffect(()=>{setStorageFailed(!persistDemoState(state))},[state]);
 const updateViewer=(patch:Partial<DemoViewer>)=>setState(current=>({...current,viewers:current.viewers.map(v=>v.id===current.activeId?{...v,...patch}:v)}));
 const toggle=(field:'favorites'|'watched',id:string)=>{if(!active)return;updateViewer({[field]:active[field].includes(id)?active[field].filter(x=>x!==id):[...active[field],id]})};
 const selectViewer=(id:string)=>{setState(current=>({...current,activeId:id}));go('/home')};
 const selectedMovie=demoMovies.find(m=>m.id===route.split('/')[2]);
 const editingViewer=state.viewers.find(v=>v.id===route.split('/')[3]);
 const isProfiles=route.startsWith('/profiles')||(!active&&route!=='/search'&&route!=='/files'&&route!=='/series'&&route!=='/anime'&&!route.startsWith('/releases')&&!route.startsWith('/downloads'));
 const favoriteMovies=demoMovies.filter(m=>active?.favorites.includes(m.id));
 const filtered=demoMovies.filter(m=>(genre==='all'||m.genre===genre)&&normalize(m.title).includes(normalize(query))).sort((a,b)=>sort==='year'?b.year-a.year:sort==='rating'?Number(b.rating)-Number(a.rating):a.title.localeCompare(b.title,locale));
 const visibleMovies=route==='/favorites'?favoriteMovies:filtered;
 const portrait=(movie:DemoMovie,wide=false)=><a className={'media-card'+(wide?' landscape':'')} href={'#/movie/'+movie.id} key={movie.id} aria-label={movie.title+' ('+movie.year+')'}><div className="media-art"><img src={'/mock-art/'+movie.id+(wide?'-wide':'')+'.svg'} alt="" loading="lazy"/>{active?.watched.includes(movie.id)&&<span className="seen-badge" aria-label={t('Watched')}><Icon name="check"/></span>}<span className="card-hover"><Icon name="play"/></span>{wide&&movie.progress&&!active?.watched.includes(movie.id)&&<span className="media-progress"><span style={{width:movie.progress+'%'}}/></span>}</div><span className="media-title">{movie.title}</span><span className="media-year">{movie.year}</span></a>;
 const row=(heading:Parameters<typeof t>[0],movies:DemoMovie[],wide=false)=>movies.length>0?<section className="media-section"><h2><a href="#/movies">{t(heading)} <span aria-hidden="true">›</span></a></h2><div className={'media-row'+(wide?' wide-row':'')}>{movies.map(m=>portrait(m,wide))}</div></section>:null;
 return <main className="demo-app">
  <SideNavigation />
  <AppHeader title="Library" viewer={active??null}/>
  <div className="demo-indicator"><span>{t(route.startsWith('/downloads')?'Live download tracking':route.startsWith('/releases')?'Live release search':route==='/search'?'Live movie metadata':route==='/files'?'Local file inventory':'Mock data · Local prototype')}</span></div>
  {storageFailed&&<p className="demo-notice" role="alert">{t('Browser storage is unavailable. Changes will last only until this page closes.')}</p>}
  {notice&&<p className="demo-notice" role="status">{notice}</p>}
  {isProfiles?<section className="profile-screen">
   {route.startsWith('/profiles/edit/')&&editingViewer?<ProfileForm initial={editingViewer} onCancel={()=>go('/profiles')} onSave={(name,photo)=>{setState(current=>({...current,viewers:current.viewers.map(v=>v.id===editingViewer.id?{...v,name,photo}:v)}));go('/profiles')}}/>:route==='/profiles/new'&&state.viewers.length<12?<ProfileForm onCancel={()=>go('/profiles')} onSave={(name,photo)=>{const id='viewer-'+crypto.randomUUID();setState(current=>({...current,viewers:[...current.viewers,{id,name,photo,color:'#237eaa',favorites:[],watched:[]}],activeId:id}));go('/home')}}/>:<>
    <h1>{t('Who is watching?')}</h1><p>{t('Choose a viewer to explore the prototype.')}</p><div className={'profile-grid'+(managing?' managing':'')}>{state.viewers.map(v=><div className="profile-entry" key={v.id}><button className="profile-choice" onClick={()=>managing?go('/profiles/edit/'+v.id):selectViewer(v.id)}><ProfileAvatar viewer={v}/><span>{v.name}</span></button><button className="profile-edit" aria-label={t('Edit profile for {name}',{name:v.name})} onClick={()=>go('/profiles/edit/'+v.id)}><Pencil/></button></div>)}{state.viewers.length<12&&<a className="profile-choice add-profile" href="#/profiles/new"><span className="profile-avatar"><Icon name="plus"/></span><span>{t('Add profile')}</span></a>}</div><button className="manage-profiles secondary" onClick={()=>setManaging(value=>!value)}>{t(managing?'Done':'Manage profiles')}</button><p className="profile-disclaimer">{t('Demo profiles personalize this browser only. They are not accounts or access controls.')}</p><a className="demo-text-link" href="/settings">{t('Real settings')}</a>
   </>}
  </section>:route.startsWith('/downloads')?<DownloadsPage key={route} acquisitionId={route.split('/')[2]}/>:route.startsWith('/releases')?<ReleaseSearch/>:route==='/search'?<MovieSearch/>:route==='/files'?<LibraryFiles/>:route==='/series'||route==='/anime'?<div className="demo-content planned-library"><h1>{t(route==='/series'?'Series':'Anime')}</h1><p>{t('Series and anime catalog support is planned. No live records or external requests are shown yet.')}</p></div>:!active?null:route==='/home'?<div className="demo-content">
   <section className="media-section"><h1>{t('My media')}</h1><div className="library-tiles"><a href="#/movies" className="library-tile"><img src="/mock-art/wild-north-wide.svg" alt=""/><strong>{t('Movies')}</strong></a><a href="#/favorites" className="library-tile"><img src="/mock-art/deep-blue-wide.svg" alt=""/><strong>{t('Favorites')}</strong></a><a href="#/downloads" className="library-tile download-tile"><Icon name="download"/><strong>{t('Downloads')}</strong></a></div></section>
   {row('Continue watching',demoMovies.filter(m=>m.progress&&!active.watched.includes(m.id)),true)}
   {row('Latest movies',demoMovies)}
   {favoriteMovies.length>0&&row('Your favorites',favoriteMovies)}
  </div>:route==='/movies'||route==='/favorites'?<div className="demo-content">
   <h1>{t(route==='/favorites'?'Favorites':'Movies')}</h1>
   {route!=='/favorites'&&<div className="library-toolbar"><div className="movie-search"><Icon name="search"/><label className="visually-hidden" htmlFor="demo-search">{t('Search movies')}</label><input id="demo-search" type="search" value={query} onChange={e=>setQuery(e.target.value)} placeholder={t('Search movies')}/></div><label>{t('Genre')}<select aria-label={t('Genre')} value={genre} onChange={e=>setGenre(e.target.value)}><option value="all">{t('All genres')}</option>{(['Science fiction','Drama','Adventure','Thriller'] as const).map(g=><option key={g} value={g}>{t(g)}</option>)}</select></label><label>{t('Sort by')}<select aria-label={t('Sort by')} value={sort} onChange={e=>setSort(e.target.value)}><option value="title">{t('Title')}</option><option value="year">{t('Year')}</option><option value="rating">{t('Rating')}</option></select></label></div>}
   <p className="result-count">{t(visibleMovies.length===1?'{count} movie':'{count} movies',{count:visibleMovies.length})}</p>
   {visibleMovies.length?<div className="poster-grid">{visibleMovies.map(m=>portrait(m))}</div>:<div className="demo-empty"><Icon name={route==='/favorites'?'heart':'search'}/><h2>{t(route==='/favorites'?'No favorites yet':'No movies found')}</h2><p>{t(route==='/favorites'?'Add favorites from a movie’s details.':'Try a different title or genre.')}</p><a href="#/movies" className="button">{t('Browse movies')}</a></div>}
  </div>:route.startsWith('/movie/')&&selectedMovie?<>
   <div className="movie-backdrop"><img src={'/mock-art/'+selectedMovie.id+'-wide.svg'} alt=""/></div><section className="movie-details"><img className="detail-poster" src={'/mock-art/'+selectedMovie.id+'.svg'} alt=""/><div className="detail-copy"><a href="#/movies" className="detail-back"><Icon name="back"/>{t('Movies')}</a><h1>{selectedMovie.title}</h1><div className="detail-meta"><span>{selectedMovie.year}</span><span>{selectedMovie.minutes} min</span><span className="quality-tag">{selectedMovie.resolution}</span><span className="quality-tag">{selectedMovie.format}</span><span>★ {selectedMovie.rating}</span></div><p className="detail-genre">{t(selectedMovie.genre)}</p><div className="detail-actions"><a className="button demo-primary" href={'#/player/'+selectedMovie.id}><Icon name="play"/>{t('Play')}</a><button className="round-action" aria-label={t('Favorite')} aria-pressed={active.favorites.includes(selectedMovie.id)} onClick={()=>toggle('favorites',selectedMovie.id)}><Icon name="heart"/></button><button className="round-action" aria-label={t('Watched')} aria-pressed={active.watched.includes(selectedMovie.id)} onClick={()=>toggle('watched',selectedMovie.id)}><Icon name="check"/></button></div><p className="synopsis">{selectedMovie.synopsis[locale]}</p><dl className="media-info"><dt>{t('Video')}</dt><dd>HEVC · {selectedMovie.resolution} · {selectedMovie.format}</dd><dt>{t('Audio')}</dt><dd>{selectedMovie.audio}</dd><dt>{t('Subtitles')}</dt><dd>English · Português (Brasil)</dd></dl></div></section>
   <div className="demo-content related-content">{row('More like this',demoMovies.filter(m=>m.genre===selectedMovie.genre&&m.id!==selectedMovie.id))}</div>
  </>:route==='/profile'?<div className="demo-content viewer-settings"><h1>{t('Viewer profile')}</h1><ProfileForm key={active.id} initial={active} onCancel={()=>go('/home')} onSave={(name,photo)=>{updateViewer({name,photo});setNotice(t('Profile saved in this browser.'))}}/><a className="button secondary" href="#/profiles">{t('Switch profile')}</a><p className="profile-disclaimer">{t('Demo profiles personalize this browser only. They are not accounts or access controls.')}</p></div>
  :route.startsWith('/player/')&&selectedMovie?<DemoPlayer key={selectedMovie.id} movie={selectedMovie}/>
  :<div className="demo-content demo-empty"><h1>{t('Page not found')}</h1><a href="#/home" className="button">{t('Home')}</a></div>}
 </main>;
}

function DemoPlayer({movie}:{movie:DemoMovie}){
 const {t}=useUI();const [playing,setPlaying]=useState(false);const [position,setPosition]=useState(movie.progress??0);
 return <section className="demo-player"><img src={'/mock-art/'+movie.id+'-wide.svg'} alt=""/><div className="player-heading"><a href={'#/movie/'+movie.id} className="icon-button" aria-label={t('Back')}><Icon name="back"/></a><h1>{movie.title}</h1></div><div className="player-simulation"><Icon name="play"/><p>{t('Playback simulation — no video is being streamed.')}</p></div><div className="player-controls"><button className="icon-button" onClick={()=>setPlaying(v=>!v)} aria-label={t(playing?'Pause':'Play')}><Icon name={playing?'pause':'play'}/></button><label className="visually-hidden" htmlFor="demo-position">{t('Playback position')}</label><input id="demo-position" type="range" min="0" max="100" value={position} onChange={e=>setPosition(Number(e.target.value))}/><span>{Math.round(movie.minutes*position/100)} / {movie.minutes} min</span><span role="status">{t(playing?'Simulating playback':'Paused')}</span></div></section>
}
