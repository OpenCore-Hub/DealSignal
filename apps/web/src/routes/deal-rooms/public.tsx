import { useState } from "react";
import { useParams } from "react-router";
import {
  FileText,
  Folder,
  Lock,
  Check,
  ArrowRight,
  ShieldCheck,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import type { PublicDealRoomView } from "@/lib/api";

type ViewState =
  | { stage: "email"; loading: boolean; error: string | null }
  | { stage: "nda"; loading: boolean; error: string | null }
  | { stage: "room"; loading: boolean; error: string | null };

export function PublicDealRoomPage() {
  const { slug } = useParams<{ slug: string }>();
  const { t } = useTranslation("dealRooms");
  const [email, setEmail] = useState("");
  const [view, setView] = useState<PublicDealRoomView | null>(null);
  const [state, setState] = useState<ViewState>({ stage: "email", loading: false, error: null });

  const setStage = (stage: ViewState["stage"], overrides?: Partial<Omit<ViewState, "stage">>) => {
    setState({ stage, loading: false, error: null, ...overrides } as ViewState);
  };

  const handleLookup = async () => {
    if (!slug || !email.trim()) return;
    setStage("email", { loading: true });
    try {
      const res = await api.getPublicDealRoom(slug, email.trim());
      setView(res);
      if (!res.member) {
        setStage("email", { error: t("public.accessDenied") });
      } else if (res.room.ndaEnabled && res.member.ndaStatus !== "signed") {
        setStage("nda");
      } else if (res.member.status !== "active") {
        setStage("email", { error: t("public.accessPending") });
      } else {
        setStage("room");
      }
    } catch (e) {
      setStage("email", { error: e instanceof Error ? e.message : t("public.loadFailed") });
    }
  };

  const handleSignNda = async () => {
    if (!slug || !email.trim()) return;
    setStage("nda", { loading: true });
    try {
      await api.signDealRoomNDA(slug, { email: email.trim() });
      await handleLookup();
    } catch (e) {
      setStage("nda", { error: e instanceof Error ? e.message : t("public.ndaFailed") });
    }
  };

  const openDocument = (documentId: string) => {
    // Public documents are viewed via the authenticated viewer route because
    // the workspace document viewer already supports public link sessions.
    // For deal rooms we open in a new tab using the same viewer path.
    window.open(`/viewer/${documentId}?room=${slug}&email=${encodeURIComponent(email.trim())}`, "_blank");
  };

  const renderGate = () => (
    <div className="flex min-h-screen items-center justify-center bg-muted/30 p-6">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {view ? view.room.name : t("public.title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {state.error && (
            <p className="text-sm text-destructive" role="alert">
              {state.error}
            </p>
          )}
          {state.stage === "email" && (
            <>
              <p className="text-body text-muted-foreground">{t("public.emailDescription")}</p>
              <div className="space-y-2">
                <Label htmlFor="public-email">{t("public.email")}</Label>
                <Input
                  id="public-email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t("public.emailPlaceholder")}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") void handleLookup();
                  }}
                />
              </div>
              <Button className="w-full gap-1" onClick={() => void handleLookup()} disabled={state.loading || !email.trim()}>
                {t("public.continue")}
                <ArrowRight size={16} />
              </Button>
            </>
          )}
          {state.stage === "nda" && (
            <>
              <div className="flex items-start gap-3 rounded-md border border-border p-3">
                <ShieldCheck size={20} className="mt-0.5 text-primary" />
                <p className="text-sm text-muted-foreground">{t("public.ndaDescription")}</p>
              </div>
              <Button className="w-full gap-1" onClick={() => void handleSignNda()} disabled={state.loading}>
                <Check size={16} />
                {t("public.signNda")}
              </Button>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );

  if (!view || state.stage !== "room") {
    return renderGate();
  }

  const visibleFolders = view.folders;
  const visibleDocs = view.documents.filter((fd) => fd.permission !== "none");

  return (
    <div className="min-h-screen bg-muted/30 p-6">
      <div className="mx-auto max-w-5xl space-y-6">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-h1">{view.room.name}</h1>
            <p className="text-body text-muted-foreground">{view.room.description}</p>
          </div>
          {view.room.ndaEnabled && (
            <Badge variant="secondary" className="gap-1 w-fit">
              <Lock size={12} />
              {t("ndaEnabled")}
            </Badge>
          )}
        </div>

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div className="space-y-4 lg:col-span-1">
            <Card>
              <CardHeader>
                <CardTitle className="text-h3 flex items-center gap-2">
                  <Folder size={18} />
                  {t("detail.folders")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                {visibleFolders.length === 0 ? (
                  <p className="text-sm text-muted-foreground">{t("detail.noFolders")}</p>
                ) : (
                  <ul className="space-y-2">
                    {visibleFolders.map((folder) => (
                      <li key={folder.path} className="flex items-center gap-2 text-sm">
                        <Folder size={16} className="text-muted-foreground" />
                        {folder.name}
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          </div>

          <div className="lg:col-span-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-h3 flex items-center gap-2">
                  <FileText size={18} />
                  {t("detail.documents")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                {visibleDocs.length === 0 ? (
                  <p className="text-sm text-muted-foreground">{t("public.noDocuments")}</p>
                ) : (
                  <div className="space-y-4">
                    {visibleDocs.map((fd) => (
                      <div key={fd.folder}>
                        <h4 className="mb-2 text-sm font-medium text-muted-foreground">
                          {folderName(fd.folder, view.folders)}
                        </h4>
                        <ul className="space-y-1">
                          {fd.documents.map((doc) => (
                            <li key={doc.id}>
                              <Button
                                variant="ghost"
                                className="h-auto w-full justify-start gap-2 px-2 py-2 font-normal"
                                onClick={() => openDocument(doc.document_id)}
                              >
                                <FileText size={16} className="text-muted-foreground" />
                                <span className="flex-1 text-left text-sm">{doc.title}</span>
                                <Badge variant="outline">{t(`public.permissions.${fd.permission}`)}</Badge>
                              </Button>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}

function folderName(path: string, folders: { path: string; name: string }[]): string {
  return folders.find((f) => f.path === path)?.name ?? path;
}
