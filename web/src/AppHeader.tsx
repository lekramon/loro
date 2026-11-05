import { useEffect, useState } from 'react';
import { readDemoState, type DemoViewer } from './mockData';
import { ProfileAvatar } from './ProfileForm';
import { InterfacePreferences, useUI } from './ui';
import type { MessageKey } from './messages';
export function AppHeader({title,viewer:override}:{title:MessageKey;viewer?:DemoViewer|null}) {
 const {t}=useUI();const [state,setState]=useState(readDemoState);
 useEffect(()=>{const refresh=()=>setState(readDemoState());window.addEventListener('piratarr:viewer-change',refresh);window.addEventListener('storage',refresh);return()=>{window.removeEventListener('piratarr:viewer-change',refresh);window.removeEventListener('storage',refresh)}},[]);
 const viewer=override===undefined?state.viewers.find(v=>v.id===state.activeId):override;
 return <header className="app-header"><span className="header-context">{t(title)}</span><div className="header-utilities"><InterfacePreferences/><a className="header-viewer" href={viewer?'/preview#/profile':'/preview#/profiles'} aria-label={t('Viewer profile')+(viewer?': '+viewer.name:'')} title={viewer?.name??t('Viewer profile')}><ProfileAvatar viewer={viewer??{name:'L',color:'#237eaa'}}/></a></div></header>;
}
