import type { Metadata } from "next";
import "./globals.css";
import { NotificationsProvider } from "../components/notifications-provider";
import { I18nProvider } from "../lib/i18n";

export const metadata: Metadata = {
  title: "AI Game Master",
  description: "AI-led game mastering with operator control, player portals, and cinematic session surfaces.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>
        <I18nProvider>
          <NotificationsProvider>{children}</NotificationsProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
