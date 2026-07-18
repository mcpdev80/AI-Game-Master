import { StudioShell } from "../../components/studio-shell";
import { SessionsCoreScreen } from "../../components/screens/sessions-core-screen";
import { fetchAdventures, fetchCampaigns, fetchCharacters, fetchDocuments, fetchSessions } from "../../lib/api";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export default async function SessionsPage() {
  const [sessions, campaigns, characters, adventures, documents] = await Promise.all([
    fetchSessions().catch(() => []),
    fetchCampaigns().catch(() => []),
    fetchCharacters().catch(() => []),
    fetchAdventures().catch(() => []),
    fetchDocuments().catch(() => []),
  ]);

  return (
    <StudioShell>
      <SessionsCoreScreen
        adventures={adventures}
        campaigns={campaigns}
        characters={characters}
        documents={documents}
        sessions={sessions}
      />
    </StudioShell>
  );
}
