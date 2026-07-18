import { SessionJoinScreen } from "../../../components/screens/session-join-screen";
import { fetchSessionJoinPreview } from "../../../lib/api";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export default async function SessionJoinPage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const preview = await fetchSessionJoinPreview(token).catch(() => null);
  return <SessionJoinScreen preview={preview} token={token} />;
}
