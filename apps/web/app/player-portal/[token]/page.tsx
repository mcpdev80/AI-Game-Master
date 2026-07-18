import { notFound } from "next/navigation";
import { PlayerPortalScreen } from "../../../components/screens/player-portal-screen";
import { fetchPlayerPortal } from "../../../lib/api";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export default async function PlayerPortalPage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const portal = await fetchPlayerPortal(token).catch(() => null);
  if (!portal) {
    notFound();
  }

  return <PlayerPortalScreen portal={portal} />;
}
