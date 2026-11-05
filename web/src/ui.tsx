import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { english, portuguese, type MessageKey } from './messages';

type Locale = 'en-US' | 'pt-BR';
type Theme = 'light' | 'dark';
const themeKey = 'piratarr.interface.theme';
const localeKey = 'piratarr.interface.locale';

function stored(key: string): string | null {
  try { return localStorage.getItem(key); } catch { return null; }
}

function persist(key: string, value: string) {
  try { localStorage.setItem(key, value); } catch { /* Preferences still work in memory when browser storage is blocked. */ }
}

const savedTheme = stored(themeKey);
const savedLocale = stored(localeKey);
const initialTheme: Theme = savedTheme === 'light' || savedTheme === 'dark' ? savedTheme : window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
const initialLocale: Locale = savedLocale === 'en-US' || savedLocale === 'pt-BR' ? savedLocale : navigator.language.toLowerCase().startsWith('pt') ? 'pt-BR' : 'en-US';

// Apply before React's first render; subsequent changes are handled by the provider.
document.documentElement.dataset.theme = initialTheme;
document.documentElement.lang = initialLocale;

type UI = {
  locale: Locale; theme: Theme;
  setLocale: (locale: Locale) => void;
  setTheme: (theme: Theme) => void;
  t: (key: MessageKey, values?: Record<string, string | number>) => string;
  errorText: (message: string) => string;
  languageName: (code: string, fallback?: string) => string;
};
const Context = createContext<UI | null>(null);

export function UIProvider({ children }: { children: ReactNode }) {
  const [locale, changeLocale] = useState<Locale>(initialLocale);
  const [theme, changeTheme] = useState<Theme>(initialTheme);
  useEffect(() => { document.documentElement.lang = locale; }, [locale]);
  useEffect(() => { document.documentElement.dataset.theme = theme; }, [theme]);
  const value = useMemo<UI>(() => {
    const messages = locale === 'pt-BR' ? portuguese : english;
    const t: UI['t'] = (key, values = {}) => messages[key].replace(/\{(\w+)\}/g, (match, name: string) => values[name] === undefined ? match : String(values[name]));
    const names = new Intl.DisplayNames([locale], { type: 'language' });
    return {
      locale, theme, t,
      setLocale(next) { persist(localeKey, next); changeLocale(next); },
      setTheme(next) { persist(themeKey, next); changeTheme(next); },
      errorText(message) { return Object.hasOwn(english, message) ? t(message as MessageKey) : t('Unable to complete the request. Please try again.'); },
      languageName(code, fallback = code) {
        if (code === 'original') return t('Original audio');
        try { const name = names.of(code); return name && name !== code ? name : fallback; } catch { return fallback; }
      },
    };
  }, [locale, theme]);
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useUI() {
  const value = useContext(Context);
  if (!value) throw new Error('UIProvider is required');
  return value;
}

export function InterfacePreferences() {
  const { t, theme, locale, setTheme, setLocale } = useUI();
  return <nav className="interface-preferences" aria-label={t('Interface preferences')}>
    <div className="preference-control"><label htmlFor="interface-theme">{t('Theme')}</label><select id="interface-theme" value={theme} onChange={event => setTheme(event.target.value as Theme)}><option value="light">{t('Light')}</option><option value="dark">{t('Dark')}</option></select></div>
    <div className="preference-control"><label htmlFor="interface-language">{t('Language')}</label><select id="interface-language" value={locale} onChange={event => setLocale(event.target.value as Locale)}><option value="en-US">English (US)</option><option value="pt-BR">Português (Brasil)</option></select></div>
  </nav>;
}
