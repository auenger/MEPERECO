import { redirect } from "next/navigation";
import { paths } from "@silieco/core/paths";

export default async function WorkspacePage({ params }: {
  params: Promise<{ workspaceSlug: string }>;
}) {
  const { workspaceSlug } = await params;
  redirect(paths.workspace(workspaceSlug).chat());
}
