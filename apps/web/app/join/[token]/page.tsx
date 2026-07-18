import { PlayerJoinScreen } from "../../../components/screens/player-join-screen";

export default async function JoinPage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  return <PlayerJoinScreen token={token} />;
}
