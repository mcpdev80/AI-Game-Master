"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { Activity, BookOpen, Calendar, Monitor, User } from "lucide-react";
import { LanguageSwitcher } from "./language-switcher";
import { useI18n, type MessageKey } from "../lib/i18n";

type StudioShellProps = {
  children: ReactNode;
};

const navItems = [
  { href: "/control-center", labelKey: "nav.controlCenter", icon: Activity },
  { href: "/library", labelKey: "nav.library", icon: BookOpen },
  { href: "/sessions", labelKey: "nav.sessions", icon: Calendar },
  { href: "/characters", labelKey: "nav.characters", icon: User },
  { href: "/player-screen", labelKey: "nav.playerScreen", icon: Monitor, external: true },
];

export function StudioShell({ children }: StudioShellProps) {
  const pathname = usePathname();
  const { t } = useI18n();

  return (
    <div className="studio-frame">
      <div className="studio-texture" aria-hidden="true" />
      <aside className="studio-sidebar">
        <div className="studio-brand">
          <h1>AI Game Master</h1>
          <p>{t("shell.subtitle")}</p>
        </div>

        <nav className="studio-nav" aria-label={t("nav.primary")}>
          {navItems.map((item) => {
            const isActive = !item.external && (pathname === item.href || pathname.startsWith(`${item.href}/`));
            const Icon = item.icon;

            return (
              <Link
                className={`studio-nav-link${isActive ? " is-active" : ""}`}
                href={item.href}
                key={item.href}
                target={item.external ? "_blank" : undefined}
              >
                <Icon size={18} />
                <span>{t(item.labelKey as MessageKey)}</span>
              </Link>
            );
          })}
        </nav>

        <div className="studio-sidebar-status">
          <div>
            <span className="status-dot status-dot--live" />
            <span>{t("shell.ready")}</span>
          </div>
          <LanguageSwitcher />
        </div>
      </aside>

      <main className="studio-main">
        <div className="studio-vignette" aria-hidden="true" />
        <div className="studio-language-mobile">
          <LanguageSwitcher compact />
        </div>
        {children}
      </main>

      <nav className="studio-mobile-nav" aria-label={t("nav.mobile")}>
        {navItems.slice(0, 4).map((item) => {
          const isActive = pathname === item.href || pathname.startsWith(`${item.href}/`);
          const Icon = item.icon;
          return (
            <Link className={`studio-mobile-link${isActive ? " is-active" : ""}`} href={item.href} key={item.href}>
              <Icon size={18} />
              <span>{t(item.labelKey as MessageKey)}</span>
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
