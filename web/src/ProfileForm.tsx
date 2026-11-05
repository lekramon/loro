import { useState } from 'react';
import type { DemoViewer } from './mockData';
import { useUI } from './ui';
export function ProfileAvatar({viewer}:{viewer:Pick<DemoViewer,'name'|'color'|'photo'>}) {
 return <span className="profile-avatar" aria-hidden="true" style={{background:viewer.color}}>{viewer.photo?<img src={viewer.photo} alt=""/>:<span>{viewer.name.slice(0,1).toUpperCase() || 'P'}</span>}</span>;
}
export function Pencil(){return <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true"><path d="m15 4 5 5M4 20l5-1L21 7l-5-5L4 14v6Z"/></svg>}
export function ProfileForm({initial,onSave,onCancel}:{initial?:DemoViewer;onSave:(name:string,photo?:string)=>void;onCancel:()=>void}){
 const {t}=useUI();const [name,setName]=useState(initial?.name??'');const [photo,setPhoto]=useState(initial?.photo);const [error,setError]=useState(false);const [loading,setLoading]=useState(false);
 async function choose(file?:File){
  if(!file)return;setLoading(true);setError(false);
  let bitmap:ImageBitmap|undefined;
  try{
   if(file.size>5*1024*1024 || !['image/jpeg','image/png','image/webp'].includes(file.type))throw new Error('invalid');
   bitmap=await createImageBitmap(file);
   if(!bitmap.width||!bitmap.height||bitmap.width>8192||bitmap.height>8192)throw new Error('invalid');
   const canvas=document.createElement('canvas');canvas.width=256;canvas.height=256;const context=canvas.getContext('2d');if(!context)throw new Error('invalid');
   const size=Math.min(bitmap.width,bitmap.height);context.drawImage(bitmap,(bitmap.width-size)/2,(bitmap.height-size)/2,size,size,0,0,256,256);
   const value=canvas.toDataURL('image/jpeg',0.85);if(value.length>262144)throw new Error('invalid');setPhoto(value);
  }catch{setError(true)}finally{bitmap?.close();setLoading(false)}
 }
 return <form className="profile-form" onSubmit={e=>{e.preventDefault();if(name.trim()&&!loading)onSave(name.trim(),photo)}}><h2>{t(initial?'Edit profile':'Add profile')}</h2>
 <div className="photo-editor"><ProfileAvatar viewer={{name,color:initial?.color??'#237eaa',photo}}/><label className="photo-picker" htmlFor="viewer-photo"><Pencil/><span>{t('Choose photo')}</span></label><input id="viewer-photo" type="file" accept="image/jpeg,image/png,image/webp" disabled={loading} onChange={e=>{void choose(e.target.files?.[0]);e.target.value=''}}/>{photo&&<button className="secondary" type="button" disabled={loading} onClick={()=>setPhoto(undefined)}>{t('Remove photo')}</button>}</div>
 <p className="hint">{t('JPEG, PNG or WebP, up to 5 MB. Stored in this browser only.')}</p>{loading&&<p role="status">{t('Preparing photo…')}</p>}{error&&<p role="alert">{t('Choose a valid JPEG, PNG or WebP image up to 5 MB.')}</p>}
 <label htmlFor="viewer-name">{t('Display name')}</label><input id="viewer-name" value={name} onChange={e=>setName(e.target.value)} maxLength={30} required autoComplete="off"/><div className="actions"><button type="submit" disabled={!name.trim()||loading}>{t('Save profile')}</button><button type="button" className="secondary" disabled={loading} onClick={onCancel}>{t('Cancel')}</button></div></form>
}
