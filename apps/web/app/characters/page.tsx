import { StudioShell } from "../../components/studio-shell";
import { CharactersScreen } from "../../components/screens/characters-screen";
import { fetchAdventures, fetchCampaigns, fetchCharacters, fetchDocuments, fetchPlayerPortal } from "../../lib/api";

export default async function CharactersPage({
  searchParams,
}: {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
}) {
  const resolvedSearchParams = searchParams ? await searchParams : {};
  const portalTokenValue = resolvedSearchParams.portal_token;
  const portalToken = Array.isArray(portalTokenValue) ? portalTokenValue[0] : portalTokenValue;
  const [characters, campaigns, adventures, documents] = await Promise.all([
    fetchCharacters().catch(() => []),
    fetchCampaigns().catch(() => []),
    fetchAdventures().catch(() => []),
    fetchDocuments().catch(() => []),
  ]);
  const portal = portalToken ? await fetchPlayerPortal(portalToken).catch(() => null) : null;

  return (
    <StudioShell>
      <CharactersScreen
        adventures={adventures}
        campaigns={campaigns}
        characters={characters}
        documents={documents}
        initialBuilderSeed={
          portal
            ? {
                portalToken: portal.token,
                returnPath: `/player-portal/${portal.token}`,
                playerSlotId: portal.player_slot.id,
                playerName: portal.player_slot.display_name,
                campaignId: portal.session.campaign_id,
                rulesetWork: portal.session.ruleset_work,
                rulesetVersion: portal.session.ruleset_version,
              }
            : null
        }
      />
    </StudioShell>
  );
}
