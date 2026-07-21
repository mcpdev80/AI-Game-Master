import { StudioShell } from "../../components/studio-shell";
import { ControlCenterScreen } from "../../components/screens/control-center-screen";
import { apiGet, fetchLLMGatewayStatus, fetchPlayerLinks, fetchSessions, type LLMGatewayStatus } from "../../lib/api";
import { getWebBuildInfo, type AppBuildInfo } from "../../lib/build-info";

type SummaryResponse = {
  services: { name: string; status: string }[];
  counts: Record<string, number>;
  llm: { base_url?: string; model?: string };
	tts: { provider?: string; model?: string };
	stt: { provider?: string; model?: string };
  build?: AppBuildInfo;
  llm_gateway?: LLMGatewayStatus;
};

export default async function ControlCenterPage() {
  const summary: SummaryResponse = await apiGet<SummaryResponse>("/api/system/summary").catch(() => ({
    services: [],
    counts: {},
    llm: {},
		tts: {},
		stt: {},
    llm_gateway: undefined,
  }));
  const sessions = await fetchSessions().catch(() => []);
  const llmGateway = await fetchLLMGatewayStatus().catch(() => summary.llm_gateway ?? null);
  const liveSession = sessions.find((session) => session.status === "live") ?? sessions[0] ?? null;
  const playerLinks = liveSession ? await fetchPlayerLinks(liveSession.id).catch(() => []) : [];
  const webBuild = getWebBuildInfo();

  return (
    <StudioShell>
      <ControlCenterScreen apiBuild={summary.build} counts={summary.counts} llm={summary.llm} llmGateway={llmGateway ?? undefined} playerLinks={playerLinks} services={summary.services} sessions={sessions} stt={summary.stt} tts={summary.tts} webBuild={webBuild} />
    </StudioShell>
  );
}
