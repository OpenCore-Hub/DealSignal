import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { DocumentsTable } from "@/components/documents/DocumentsTable";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";

type NDATemplate = {
  id: string;
  name: string;
  source_document_id: string;
  response_count: number;
  link_count: number;
  status: string;
  updated_at: string;
};

type NDAResponse = {
  id: string;
  email: string;
  signer_name: string;
  certificate_id: string;
  has_signed_file: boolean;
  signed_at: string;
};

export function AgreementDocumentsPage() {
  const { t } = useTranslation("agreementDocuments");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [templates, setTemplates] = useState<NDATemplate[]>([]);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const [responses, setResponses] = useState<NDAResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadTemplates = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.listNDATemplates();
      setTemplates(res.data ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("agreementDocuments:page.loadError"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadTemplates();
  }, [loadTemplates]);

  useEffect(() => {
    if (!selectedTemplateId) {
      setResponses([]);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const res = await api.listNDATemplateResponses(selectedTemplateId);
        if (!cancelled) setResponses(res.data ?? []);
      } catch {
        if (!cancelled) setResponses([]);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selectedTemplateId]);

  const downloadResponse = async (responseId: string, certificateId: string) => {
    if (!workspaceSlug) return;
    const res = await fetch(
      `/api/v1/workspaces/${encodeURIComponent(workspaceSlug)}/nda/responses/${responseId}/download`,
      { credentials: "include" }
    );
    if (!res.ok) return;
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `nda-signed-${certificateId}.pdf`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="space-y-8">
      <div className="space-y-1">
        <h1 className="text-h1">{t("agreementDocuments:page.title")}</h1>
        <p className="text-body text-muted-foreground">
          {t("agreementDocuments:page.description")}
        </p>
      </div>

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-lg font-medium">{t("agreementDocuments:page.templatesTitle")}</h2>
          <Button variant="outline" size="sm" onClick={() => void loadTemplates()}>
            {t("agreementDocuments:page.refresh")}
          </Button>
        </div>
        {loading && <p className="text-sm text-muted-foreground">{t("agreementDocuments:page.loading")}</p>}
        {error && <p className="text-sm text-destructive">{error}</p>}
        {!loading && templates.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("agreementDocuments:page.templatesEmpty")}</p>
        )}
        {templates.length > 0 && (
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full text-sm">
              <thead className="bg-muted/40 text-left">
                <tr>
                  <th className="px-3 py-2 font-medium">{t("agreementDocuments:page.colName")}</th>
                  <th className="px-3 py-2 font-medium">{t("agreementDocuments:page.colLinks")}</th>
                  <th className="px-3 py-2 font-medium">{t("agreementDocuments:page.colSignatures")}</th>
                  <th className="px-3 py-2 font-medium">{t("agreementDocuments:page.colActions")}</th>
                </tr>
              </thead>
              <tbody>
                {templates.map((tpl) => (
                  <tr key={tpl.id} className="border-t">
                    <td className="px-3 py-2">{tpl.name}</td>
                    <td className="px-3 py-2">{tpl.link_count}</td>
                    <td className="px-3 py-2">{tpl.response_count}</td>
                    <td className="px-3 py-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setSelectedTemplateId(tpl.id)}
                      >
                        {t("agreementDocuments:page.viewSignatures")}
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {selectedTemplateId && (
          <div className="space-y-2 rounded-md border p-3">
            <h3 className="text-sm font-medium">{t("agreementDocuments:page.signaturesTitle")}</h3>
            {responses.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("agreementDocuments:page.signaturesEmpty")}</p>
            ) : (
              <ul className="space-y-2">
                {responses.map((r) => (
                  <li key={r.id} className="flex flex-wrap items-center justify-between gap-2 text-sm">
                    <span>
                      {r.signer_name || "—"} · {r.email || "—"} · {r.certificate_id}
                    </span>
                    {r.has_signed_file && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => void downloadResponse(r.id, r.certificate_id)}
                      >
                        {t("agreementDocuments:page.downloadSigned")}
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-medium">{t("agreementDocuments:page.documentsTitle")}</h2>
        <p className="text-sm text-muted-foreground">{t("agreementDocuments:page.documentsHint")}</p>
        <DocumentsTable category="agreement" />
      </section>
    </div>
  );
}
