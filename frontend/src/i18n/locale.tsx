import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';
import { uiMessages } from './messages';
import type { Locale, LocaleContextValue } from './types';

export const LOCALE_STORAGE_KEY = 'tgadmin-locale';

const defaultContext: LocaleContextValue = {
  locale: 'ru',
  setLocale: () => undefined,
  t: (key, fallback) => fallback ?? key,
};

const LocaleContext = createContext<LocaleContextValue>(defaultContext);

function readStoredLocale(): Locale {
  try {
    return localStorage.getItem(LOCALE_STORAGE_KEY) === 'en' ? 'en' : 'ru';
  } catch {
    return 'ru';
  }
}

export function LocaleProvider({
  children,
  initialLocale,
}: {
  children: ReactNode;
  initialLocale?: Locale;
}) {
  const [locale, setLocaleState] = useState<Locale>(() => initialLocale ?? readStoredLocale());

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    try {
      localStorage.setItem(LOCALE_STORAGE_KEY, next);
    } catch {
      // localStorage может быть недоступен в приватном режиме.
    }
  }, []);

  const t = useCallback(
    (key: string, fallback?: string) => uiMessages[locale][key] ?? fallback ?? key,
    [locale],
  );

  const value = useMemo(() => ({ locale, setLocale, t }), [locale, setLocale, t]);
  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>;
}

export function useLocale(): LocaleContextValue {
  return useContext(LocaleContext);
}
