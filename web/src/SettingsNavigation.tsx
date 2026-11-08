import { useUI } from './ui';

export const settingsSections = [
  ['media-management', 'Media Management'], ['profiles', 'Profiles'], ['quality', 'Quality'], ['custom-formats', 'Custom Formats'],
  ['indexers', 'Indexers'], ['download-clients', 'Download Clients'], ['import-lists', 'Import Lists'], ['subtitles', 'Subtitles'],
  ['general', 'General'], ['system', 'System'],
] as const;

export function settingsPath(path: string) {
  const section = path.replace(/^\/settings\/?/, '').split('/')[0];
  return settingsSections.some(([key]) => key === section) ? section : 'media-management';
}

export function SettingsNavigation({ path }: { path: string }) {
  const { t } = useUI();
  const active = settingsPath(path);
  return <section className="settings-tabs" aria-labelledby="settings-tabs-heading"><h2 id="settings-tabs-heading">{t('Settings')}</h2><nav aria-label={t('Settings sections')}>
    {settingsSections.map(([key, label]) => <a key={key} href={key === 'media-management' ? '/settings' : `/settings/${key}`} aria-current={active === key ? 'page' : undefined}>{t(label)}</a>)}
  </nav></section>;
}
