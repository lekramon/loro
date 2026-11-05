import { MetadataSettings } from './MetadataSettings';
import { useState } from 'react';
import { FolderPicker } from './FolderPicker';
import { useUI } from './ui';
import { api, ApiError, type FieldError, type Settings, type SettingsInput, type SetupState } from './api';

export function SetupWizard({ setup, onSaved, initial, onCancel }: { setup: SetupState; onSaved: (settings: Settings) => void; initial?: Settings; onCancel?: () => void }) {
  const { t, errorText, languageName } = useUI();
  const [step, setStep] = useState(0);
 const [browsing,setBrowsing]=useState<'downloadDirectory'|'movieDirectory'>(); const [creating,setCreating]=useState(false);
 const formats=['remux','bluray','web-dl','webrip','hdtv','dvd']; const containers=['mkv','mp4','avi','mov','m4v','ts'];
  const [input, setInput] = useState<SettingsInput>(() => initial ? {
    directories: { ...initial.directories },
    profile: { ...initial.profile, acceptedFormats: [...(initial.profile.acceptedFormats ?? formats)], acceptedContainers: [...(initial.profile.acceptedContainers ?? containers)], acceptedResolutions: [...initial.profile.acceptedResolutions], audioPreferences: [...initial.profile.audioPreferences] },
  } : { directories: { downloadDirectory: '', movieDirectory: '' }, profile: { acceptedFormats: [...formats], acceptedContainers: [...containers], acceptedResolutions: [...setup.supportedResolutions], preferredResolution: '', audioPreferences: [] } });
  const [search, setSearch] = useState('');
  const [fields, setFields] = useState<FieldError[]>([]);
  const [message, setMessage] = useState('');
  const [saving, setSaving] = useState(false);
  const [conflict, setConflict] = useState(false);
  const normalize = (value: string) => value.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();
  const choices = [{ code: 'original', name: languageName('original') }, ...setup.languages.map(choice => ({ ...choice, name: languageName(choice.code, choice.name) }))];
  const audioName = (code: string) => choices.find(choice => choice.code === code)?.name ?? code;
  const fieldErrors = (field: string) => fields.filter(item => item.field === field);
  const feedback = (field: string) => <>{fieldErrors(field).map((error, index) => <p className="field-error" key={index}>{errorText(error.message)}</p>)}</>;
  const updateDirectory = (field: 'downloadDirectory' | 'movieDirectory', value: string) => {
    setInput(current => ({ ...current, directories: { ...current.directories, [field]: value } }));
    setFields(current => current.filter(item => item.field !== `directories.${field}`));
  };
  const toggleResolution = (value: string) => setInput(current => {
    const accepted = current.profile.acceptedResolutions.includes(value) ? current.profile.acceptedResolutions.filter(item => item !== value) : setup.supportedResolutions.filter(item => item === value || current.profile.acceptedResolutions.includes(item));
    return { ...current, profile: { ...current.profile, acceptedResolutions: accepted, preferredResolution: accepted.includes(current.profile.preferredResolution) ? current.profile.preferredResolution : '' } };
  });
  const toggleAudio = (code: string) => setInput(current => ({ ...current, profile: { ...current.profile, audioPreferences: current.profile.audioPreferences.includes(code) ? current.profile.audioPreferences.filter(item => item !== code) : [...current.profile.audioPreferences, code] } }));
  const moveAudio = (index: number, delta: number) => setInput(current => {
    const ranked = [...current.profile.audioPreferences];
    [ranked[index], ranked[index + delta]] = [ranked[index + delta], ranked[index]];
    return { ...current, profile: { ...current.profile, audioPreferences: ranked } };
  });
  const proceed = () => {
    const errors: FieldError[] = [];
    if (step === 0) {
      if (!input.directories.downloadDirectory.trim()) errors.push({ field: 'directories.downloadDirectory', message: 'Enter your existing download directory.' });
      if (!input.directories.movieDirectory.trim()) errors.push({ field: 'directories.movieDirectory', message: 'Enter your existing movie directory.' });
    }
    if (step === 1) {
 if (!input.profile.acceptedFormats.length || !input.profile.acceptedContainers.length) errors.push({field:'profile.acceptedFormats',message:'Choose at least one distinct supported format or container.'});
      if (input.profile.acceptedResolutions.length === 0) errors.push({ field: 'profile.acceptedResolutions', message: 'Choose at least one accepted resolution.' });
      if (!input.profile.preferredResolution) errors.push({ field: 'profile.preferredResolution', message: 'Choose a preferred resolution.' });
    }
    if (step === 2 && input.profile.audioPreferences.length === 0) errors.push({ field: 'profile.audioPreferences', message: 'Choose at least one language or original audio.' });
    setFields(errors);
    if (errors.length === 0) { setStep(step + 1); setMessage(''); }
  };
  const save = async () => {
    if (saving || conflict) return;
    setSaving(true); setMessage(''); setFields([]);
    try {
      const saved = await api<Settings>(initial ? '/settings' : '/setup', { method: initial ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json', 'X-Piratarr-Request': '1' }, body: JSON.stringify(initial ? { ...input, revision: initial.revision } : input) });
      onSaved(saved);
    } catch (error) {
      if (error instanceof ApiError) {
        setMessage(error.message); setFields(error.fieldErrors);
        if (error.status === 409) setConflict(true);
        if (error.status === 422) {
          const first = error.fieldErrors[0]?.field ?? '';
          setStep(first.startsWith('directories.') ? 0 : first === 'profile.audioPreferences' ? 2 : 1);
        }
      } else {
        setMessage('Unable to confirm whether your choices were saved. Reload the saved settings before trying again.');
        setConflict(true);
      }
    } finally { setSaving(false); }
  };
  const headings = initial ? ['Edit your local folders', 'Edit your picture quality', 'Edit and rank your audio', 'Review your changes'] as const : ['Choose your local folders', 'Choose your picture quality', 'Choose and rank your audio', 'Review your setup'] as const;
  return <>
 {browsing && <FolderPicker value={input.directories[browsing]} onClose={()=>setBrowsing(undefined)} onChoose={path=>{updateDirectory(browsing,path);setBrowsing(undefined)}}/>}
    <ol className="wizard-progress" aria-label={t('Setup progress')}>{(['Folders', 'Quality', 'Audio', 'Review'] as const).map((label,index)=><li key={label} className={index===step?'current':index<step?'complete':''} aria-current={index===step?'step':undefined}><span>{index<step?'✓':index+1}</span>{t(label)}</li>)}</ol>
    <span className="badge">{initial ? t('Edit settings · revision {revision}', { revision: initial.revision }) : t('First-run setup')} · {t('Step {step} of 4', { step: step + 1 })}</span><h2>{t(headings[step])}</h2>
    <p>{t("Your choices stay on this machine and apply to all movies.")}</p>
    {message && <div className="error-summary" role="alert">{errorText(message)}{conflict && <p><button type="button" onClick={() => window.location.reload()}>{t("Reload saved settings")}</button></p>}</div>}
    <form onSubmit={event => { event.preventDefault(); if (step === 3) void save(); else proceed(); }}>
      <fieldset disabled={saving || conflict || creating}>
        {step === 0 && <>
 <MetadataSettings />
 <button type="button" className="secondary" disabled={creating} onClick={async()=>{setCreating(true);setMessage('');try{const directories=await api<SettingsInput['directories']>('/directories/defaults',{method:'POST',headers:{'Content-Type':'application/json','X-Piratarr-Request':'1'},body:'{}'});setInput(current=>({...current,directories}));setFields([])}catch{setMessage('Folders are unavailable.')}finally{setCreating(false)}}}>{t('Create and use default folders')}</button>
          <p className="hint">{t("Enter absolute paths as seen by Piratarr. Both folders must already exist and be separate from each other and application data. Existing media stays in place.")}</p>
          <label htmlFor="download-directory">{t("Download directory")}</label><input id="download-directory" value={input.directories.downloadDirectory} onChange={event => updateDirectory('downloadDirectory', event.target.value)} autoComplete="off" spellCheck={false} aria-invalid={fieldErrors('directories.downloadDirectory').length > 0} />{feedback('directories.downloadDirectory')}<button type="button" className="secondary browse-button" onClick={()=>setBrowsing('downloadDirectory')}>{t('Browse folders')}</button>
          <label htmlFor="movie-directory">{t("Movie directory")}</label><input id="movie-directory" value={input.directories.movieDirectory} onChange={event => updateDirectory('movieDirectory', event.target.value)} autoComplete="off" spellCheck={false} aria-invalid={fieldErrors('directories.movieDirectory').length > 0} />{feedback('directories.movieDirectory')}<button type="button" className="secondary browse-button" onClick={()=>setBrowsing('movieDirectory')}>{t('Browse folders')}</button>
          <p className="hint">{t("Folders are checked when you save. Validation creates and removes only a temporary probe file.")}</p>
          {initial && <p className="hint">{t("Changing a folder updates its configured location only. Existing files are not moved or renamed.")}</p>}
        </>}
        {step === 1 && <>
          <label htmlFor="preferred-resolution">{t("Preferred resolution")}</label><select id="preferred-resolution" value={input.profile.preferredResolution} onChange={event => setInput(current => ({ ...current, profile: { ...current.profile, preferredResolution: event.target.value } }))}><option value="">{t("Choose a resolution")}</option>{input.profile.acceptedResolutions.map(value => <option key={value} value={value}>{value}</option>)}</select>{feedback('profile.preferredResolution')}
<p className="hint">{t('All options are accepted. Remove anything you do not want.')}</p>          <fieldset><legend>{t("Accepted resolutions")}</legend><div className="choice-grid">{setup.supportedResolutions.map(value => <label className="choice" key={value}><input type="checkbox" checked={input.profile.acceptedResolutions.includes(value)} onChange={() => toggleResolution(value)} />{value}</label>)}</div></fieldset>{feedback('profile.acceptedResolutions')}
{(['acceptedFormats','acceptedContainers'] as const).map(group=><fieldset key={group}><legend>{t(group==='acceptedFormats'?'Release formats':'File containers')}</legend><div className="choice-grid">{(group==='acceptedFormats'?formats:containers).map(value=><label className="choice" key={value}><input type="checkbox" checked={input.profile[group].includes(value)} onChange={()=>setInput(current=>({...current,profile:{...current.profile,[group]:current.profile[group].includes(value)?current.profile[group].filter(item=>item!==value):[...current.profile[group],value]}}))}/>{value==='bluray'?'Blu-ray encode':value.toUpperCase()}</label>)}</div>{feedback('profile.'+group)}</fieldset>)}<p className="hint">{t('Remux is a release format; MKV and MP4 are file containers.')}</p>          <p className="hint">{t("Other resolutions will remain visible with explanations during manual search.")}</p>
        </>}
        {step === 2 && <>
          <label htmlFor="language-search">{t("Search audio languages")}</label><input id="language-search" value={search} onChange={event => setSearch(event.target.value)} type="search" />
          <div className="language-list" role="group" aria-label={t('Available audio choices')}>{choices.filter(choice => normalize(`${choice.name} ${choice.code}`).includes(normalize(search))).map(choice => <label className="choice" key={choice.code}><input type="checkbox" checked={input.profile.audioPreferences.includes(choice.code)} onChange={() => toggleAudio(choice.code)} />{choice.name}</label>)}</div>{feedback('profile.audioPreferences')}
          <h3>{t("Preference order")}</h3>
          {input.profile.audioPreferences.length === 0 ? <p className="hint">{t("Select audio choices above. Nothing is preselected.")}</p> : <ol className="ranking">{input.profile.audioPreferences.map((code, index) => <li key={code}><span>{audioName(code)}</span><div><button type="button" className="secondary" disabled={index === 0} onClick={() => moveAudio(index, -1)} aria-label={t('Move {language} up', { language: audioName(code) })}>{t("Up")}</button><button type="button" className="secondary" disabled={index === input.profile.audioPreferences.length - 1} onClick={() => moveAudio(index, 1)} aria-label={t('Move {language} down', { language: audioName(code) })}>{t("Down")}</button><button type="button" className="secondary" onClick={() => toggleAudio(code)} aria-label={t('Remove {language}', { language: audioName(code) })}>{t("Remove")}</button></div></li>)}</ol>}
          <p className="hint">{t("Original audio follows each movie’s original language. Unspecified audio stays unknown.")}</p>
        </>}
        {step === 3 && <dl className="summary"><dt>{t("Download directory")}</dt><dd>{input.directories.downloadDirectory}</dd><dt>{t("Movie directory")}</dt><dd>{input.directories.movieDirectory}</dd><dt>{t("Accepted resolutions")}</dt><dd>{input.profile.acceptedResolutions.join(', ')}</dd><dt>{t("Preferred resolution")}</dt><dd>{input.profile.preferredResolution}</dd><dt>{t("Release formats")}</dt><dd>{input.profile.acceptedFormats.join(", ")}</dd><dt>{t("File containers")}</dt><dd>{input.profile.acceptedContainers.join(", ")}</dd><dt>{t("Audio preference order")}</dt><dd>{input.profile.audioPreferences.map(audioName).join(' → ')}</dd></dl>}
        <div className="actions">{step > 0 && <button type="button" className="secondary" onClick={() => { setStep(step - 1); setFields([]); }}>{t("Back")}</button>}<button type="submit">{saving ? t('Saving…') : step === 3 ? initial ? t('Save changes') : t('Save setup') : t('Continue')}</button>{onCancel && <button type="button" className="secondary" onClick={onCancel}>{t("Cancel editing")}</button>}</div>
      </fieldset>
    </form>
  </>;
}
