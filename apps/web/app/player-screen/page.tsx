import { PlayerScreenView } from "../../components/screens/player-screen-view";
import { fetchAdventures, fetchAssets, fetchCharacters, fetchDocuments, fetchPlayerLinks, fetchSessions } from "../../lib/api";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export default async function PlayerScreenPage() {
  const [sessions, documents, assets, adventures, characters] = await Promise.all([
    fetchSessions().catch(() => []),
    fetchDocuments().catch(() => []),
    fetchAssets().catch(() => []),
    fetchAdventures().catch(() => []),
    fetchCharacters().catch(() => []),
  ]);
  const session = sessions.find((item) => item.status === "live") ?? sessions[0] ?? null;
  const playerLinks = session ? await fetchPlayerLinks(session.id).catch(() => []) : [];
  return <PlayerScreenView adventures={adventures} assets={assets} characters={characters} documents={documents} playerLinks={playerLinks} session={session} />;
}
