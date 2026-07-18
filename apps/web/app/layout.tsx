import type { Metadata } from "next";
import { cookies } from "next/headers";
import "./globals.css";
import { NotificationsProvider } from "../components/notifications-provider";
import { I18nProvider, type Locale } from "../lib/i18n";

export async function generateMetadata(): Promise<Metadata> {
  const locale = (await cookies()).get("locale")?.value === "de" ? "de" : "en";
  return {
    title: "AI Game Master",
    description: locale === "de"
      ? "KI-geführte Spielleitung mit Kontrollzentrum, Spielerportalen und filmischer Sitzungsansicht."
      : "AI-led game mastering with operator control, player portals, and cinematic session surfaces.",
  };
}

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const cookieStore = await cookies();
  const initialLocale: Locale = cookieStore.get("locale")?.value === "de" ? "de" : "en";
  return (
    <html lang={initialLocale}>
      <body>
        <I18nProvider initialLocale={initialLocale}>
          <NotificationsProvider>{children}</NotificationsProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
