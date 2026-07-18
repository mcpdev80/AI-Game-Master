import { notFound } from "next/navigation";
import { ActiveSessionScreen } from "../../../components/screens/active-session-screen";
import { fetchAdventures, fetchAssets, fetchCharacters, fetchDocuments, fetchPlayerLinks, fetchSession, fetchSessionEvents } from "../../../lib/api";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export default async function SessionDetailPage({
  params,
}: {
  params: Promise<{ sessionId: string }>;
}) {
  const { sessionId } = await params;
  const session = await fetchSession(sessionId).catch(() => null);
  if (!session) {
    notFound();
  }

  const events = await fetchSessionEvents(sessionId).catch(() => []);
  const playerLinks = await fetchPlayerLinks(sessionId).catch(() => []);
  const adventures = await fetchAdventures().catch(() => []);
  const documents = await fetchDocuments().catch(() => []);
  const assets = await fetchAssets().catch(() => []);
  const characters = await fetchCharacters().catch(() => []);
  return <ActiveSessionScreen adventures={adventures} assets={assets} characters={characters} documents={documents} events={events} playerLinks={playerLinks} session={session} />;
}
