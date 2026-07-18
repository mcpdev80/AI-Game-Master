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
  tr: (english: string, german: string) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

function isLocale(value: string | null): value is Locale {
  return value === "en" || value === "de";
}

export function I18nProvider({ children, initialLocale = DEFAULT_LOCALE }: { children: ReactNode; initialLocale?: Locale }) {
  const [locale, setLocaleState] = useState<Locale>(initialLocale);

  useEffect(() => {
    const savedLocale = window.localStorage.getItem(STORAGE_KEY);
    const hasLocaleCookie = document.cookie.split(";").some((entry) => entry.trim().startsWith("locale="));
    if (!hasLocaleCookie && isLocale(savedLocale)) {
      setLocaleState(savedLocale);
      document.cookie = `locale=${savedLocale}; Path=/; Max-Age=31536000; SameSite=Lax`;
      document.documentElement.lang = savedLocale;
      return;
    }
    window.localStorage.setItem(STORAGE_KEY, initialLocale);
    document.documentElement.lang = initialLocale;
  }, [initialLocale]);

  const setLocale = useCallback((nextLocale: Locale) => {
    setLocaleState(nextLocale);
    window.localStorage.setItem(STORAGE_KEY, nextLocale);
    document.cookie = `locale=${nextLocale}; Path=/; Max-Age=31536000; SameSite=Lax`;
    document.documentElement.lang = nextLocale;
  }, []);

  const t = useCallback((key: MessageKey) => messages[locale][key] ?? messages.en[key], [locale]);
  const tr = useCallback((english: string, german: string) => (locale === "de" ? german : english), [locale]);
  const value = useMemo(() => ({ locale, setLocale, t, tr }), [locale, setLocale, t, tr]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return context;
}
