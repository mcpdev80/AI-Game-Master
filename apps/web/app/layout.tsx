import type { Metadata } from "next";
import "./globals.css";
import { NotificationsProvider } from "../components/notifications-provider";

export const metadata: Metadata = {
  title: "AI Game Master",
  description: "AI-led dungeon mastering with operator control, player portals, and cinematic session surfaces.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="de">
      <body>
        <NotificationsProvider>{children}</NotificationsProvider>
      </body>
    </html>
  );
}
