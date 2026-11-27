export type Locale = 'ru' | 'en';

export type Messages = Record<string, string>;

export type Translator = (key: string, fallback?: string) => string;

export interface LocaleContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: Translator;
}
