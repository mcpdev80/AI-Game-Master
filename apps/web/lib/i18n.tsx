"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

export type Locale = "en" | "de";

const DEFAULT_LOCALE: Locale = "en";
const STORAGE_KEY = "ai-game-master-locale";

const messages = {
  en: {
    "language.label": "Language",
    "language.english": "English",
    "language.german": "Deutsch",
    "nav.primary": "Primary navigation",
    "nav.mobile": "Mobile navigation",
    "nav.controlCenter": "Control Center",
    "nav.library": "Library",
    "nav.sessions": "Sessions",
    "nav.characters": "Characters",
    "nav.playerScreen": "Player Screen",
    "shell.subtitle": "Tabletop Studio",
    "shell.ready": "System Ready",
  },
  de: {
    "language.label": "Sprache",
    "language.english": "English",
    "language.german": "Deutsch",
    "nav.primary": "Hauptnavigation",
    "nav.mobile": "Mobile Navigation",
    "nav.controlCenter": "Kontrollzentrum",
    "nav.library": "Bibliothek",
    "nav.sessions": "Sitzungen",
    "nav.characters": "Charaktere",
    "nav.playerScreen": "Spieleransicht",
    "shell.subtitle": "Pen-&-Paper-Studio",
    "shell.ready": "System bereit",
  },
} as const;

export type MessageKey = keyof (typeof messages)["en"];

type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: MessageKey) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

function isLocale(value: string | null): value is Locale {
  return value === "en" || value === "de";
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(DEFAULT_LOCALE);

  useEffect(() => {
    const savedLocale = window.localStorage.getItem(STORAGE_KEY);
    if (isLocale(savedLocale)) {
      setLocaleState(savedLocale);
      document.documentElement.lang = savedLocale;
      return;
    }
    document.documentElement.lang = DEFAULT_LOCALE;
  }, []);

  const setLocale = useCallback((nextLocale: Locale) => {
    setLocaleState(nextLocale);
    window.localStorage.setItem(STORAGE_KEY, nextLocale);
    document.cookie = `locale=${nextLocale}; Path=/; Max-Age=31536000; SameSite=Lax`;
    document.documentElement.lang = nextLocale;
  }, []);

  const t = useCallback((key: MessageKey) => messages[locale][key] ?? messages.en[key], [locale]);
  const value = useMemo(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return context;
}
