import { redirect } from "next/navigation";

export default function SetupLegacyPage() {
  redirect("/sessions");
}
