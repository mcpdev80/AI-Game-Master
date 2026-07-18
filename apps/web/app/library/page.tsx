import { StudioShell } from "../../components/studio-shell";
import { LibraryCoreScreen } from "../../components/screens/library-core-screen";
import { fetchAdventures, fetchAssets, fetchCampaigns, fetchDocuments } from "../../lib/api";

export default async function LibraryPage() {
  const [campaigns, adventures, documents, assets] = await Promise.all([
    fetchCampaigns().catch(() => []),
    fetchAdventures().catch(() => []),
    fetchDocuments().catch(() => []),
    fetchAssets().catch(() => []),
  ]);

  return (
    <StudioShell>
      <LibraryCoreScreen adventures={adventures} assets={assets} campaigns={campaigns} documents={documents} />
    </StudioShell>
  );
}
