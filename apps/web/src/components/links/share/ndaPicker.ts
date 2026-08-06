import { api } from "@/lib/api";
import type { Document } from "@/types";

/** Workspace NDA template option for AccessTab / BundleSecurityOptions. */
export type NdaTemplateOption = {
  id: string;
  name: string;
  sourceDocumentId: string;
};

/** Document option from the workspace agreement library (协议). */
export type NdaDocumentOption = {
  id: string;
  title: string;
};

export type NdaPickerSources = {
  templates: NdaTemplateOption[];
  agreementDocs: NdaDocumentOption[];
};

type TemplateRow = {
  id: string;
  name: string;
  source_document_id: string;
};

/** Map API template rows into picker options. */
export function mapNdaTemplates(rows: TemplateRow[] | null | undefined): NdaTemplateOption[] {
  return (rows ?? []).map((tpl) => ({
    id: tpl.id,
    name: tpl.name,
    sourceDocumentId: tpl.source_document_id,
  }));
}

/**
 * Agreement-library docs eligible for NDA picker.
 * Only ready PDFs can be signed; archived/processing are excluded.
 */
export function mapAgreementDocuments(docs: Document[] | null | undefined): NdaDocumentOption[] {
  return (docs ?? [])
    .filter((d) => d.status === "ready")
    .map((d) => ({ id: d.id, title: d.title }));
}

/**
 * NDA picker document list: agreement library only.
 * Never fall back to deal-room folder docs or share-content docs — those are
 * diligence materials, not signable NDA agreements.
 */
export function resolveNdaDocumentFallback(
  agreementDocs: NdaDocumentOption[],
): NdaDocumentOption[] {
  return agreementDocs;
}

/** Accept `{ data: T[] }` or a bare array from `request` / mocks. */
function unwrapList<T>(res: unknown): T[] {
  if (Array.isArray(res)) return res as T[];
  if (res && typeof res === "object" && Array.isArray((res as { data?: unknown }).data)) {
    return (res as { data: T[] }).data;
  }
  return [];
}

/**
 * Load NDA picker sources with partial failure tolerance:
 * templates and agreement docs resolve independently so one failing call
 * does not wipe the other.
 */
export async function loadNdaPickerSources(): Promise<NdaPickerSources> {
  // Wrap in then() so sync mock/missing-fn throws become rejected settlements.
  const [tplSettled, docsSettled] = await Promise.allSettled([
    Promise.resolve().then(() => api.listNDATemplates()),
    Promise.resolve().then(() => api.getDocuments("all", "agreement")),
  ]);

  const templates =
    tplSettled.status === "fulfilled"
      ? mapNdaTemplates(unwrapList<TemplateRow>(tplSettled.value))
      : [];
  const agreementDocs =
    docsSettled.status === "fulfilled"
      ? mapAgreementDocuments(unwrapList<Document>(docsSettled.value))
      : [];

  return { templates, agreementDocs };
}
