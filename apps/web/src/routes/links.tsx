import { Navigate, useParams, useSearchParams } from "react-router";
import { documentsSharePath } from "@/lib/documentsSharePath";

/** Legacy /links list → Document Library Share tab. */
export function LinksPage() {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [searchParams] = useSearchParams();
  const documentId = searchParams.get("documentId") ?? undefined;
  const documentTitle = searchParams.get("documentTitle") ?? undefined;

  if (!workspaceSlug) {
    return <Navigate to="/" replace />;
  }

  return (
    <Navigate
      to={documentsSharePath(workspaceSlug, { documentId, documentTitle })}
      replace
    />
  );
}
