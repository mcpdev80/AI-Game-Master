"use client";

import { Languages } from "lucide-react";
import { useI18n, type Locale } from "../lib/i18n";

export function LanguageSwitcher({ compact = false }: { compact?: boolean }) {
  const { locale, setLocale, t } = useI18n();
  const options: { locale: Locale; shortLabel: string }[] = [
    { locale: "en", shortLabel: "EN" },
    { locale: "de", shortLabel: "DE" },
  ];

  return (
    <div className={`language-switcher${compact ? " language-switcher--compact" : ""}`} aria-label={t("language.label")}>
      {compact ? null : <Languages aria-hidden="true" size={16} />}
      <span className="sr-only">{t("language.label")}</span>
      {options.map((option) => (
        <button
          aria-label={option.locale === "en" ? t("language.english") : t("language.german")}
          aria-pressed={locale === option.locale}
          className={locale === option.locale ? "is-active" : ""}
          key={option.locale}
          onClick={() => setLocale(option.locale)}
          type="button"
        >
          {option.shortLabel}
        </button>
      ))}
    </div>
  );
}
