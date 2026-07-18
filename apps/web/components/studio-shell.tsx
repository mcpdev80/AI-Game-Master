"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { Activity, BookOpen, Calendar, Monitor, User } from "lucide-react";

type StudioShellProps = {
  children: ReactNode;
};

const navItems = [
  { href: "/control-center", label: "Control Center", icon: Activity },
  { href: "/library", label: "Library", icon: BookOpen },
  { href: "/sessions", label: "Sessions", icon: Calendar },
  { href: "/characters", label: "Characters", icon: User },
  { href: "/player-screen", label: "Player Screen", icon: Monitor, external: true },
];

export function StudioShell({ children }: StudioShellProps) {
  const pathname = usePathname();

  return (
    <div className="studio-frame">
      <div className="studio-texture" aria-hidden="true" />
      <aside className="studio-sidebar">
        <div className="studio-brand">
          <h1>AI Game Master</h1>
          <p>Pen & Paper Studio</p>
        </div>

        <nav className="studio-nav" aria-label="Primary">
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
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>

        <div className="studio-sidebar-status">
          <span className="status-dot status-dot--live" />
          <span>System Ready</span>
        </div>
      </aside>

      <main className="studio-main">
        <div className="studio-vignette" aria-hidden="true" />
        {children}
      </main>

      <nav className="studio-mobile-nav" aria-label="Mobile primary">
        {navItems.slice(0, 4).map((item) => {
          const isActive = pathname === item.href || pathname.startsWith(`${item.href}/`);
          const Icon = item.icon;
          return (
            <Link className={`studio-mobile-link${isActive ? " is-active" : ""}`} href={item.href} key={item.href}>
              <Icon size={18} />
              <span>{item.label}</span>
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
