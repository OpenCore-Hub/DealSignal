import { useState } from "react";
import { ShieldCheck, Download, UserMinus, Trash } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";

export function SettingsCompliancePage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");

  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);

  const valid = /^\S+@\S+\.\S+$/.test(email);

  const handleExport = async () => {
    if (!valid) return;
    setLoading(true);
    try {
      const data = await api.exportVisitorData(email);
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `visitor-export-${email}.json`;
      a.click();
      URL.revokeObjectURL(url);
      toast.success(t("compliance.exportSuccess"));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setLoading(false);
    }
  };

  const handleAnonymize = async () => {
    if (!valid) return;
    if (!window.confirm(t("compliance.anonymizeConfirm"))) return;
    setLoading(true);
    try {
      const summary = await api.anonymizeVisitorData(email);
      toast.success(t("compliance.anonymizeSuccess", { count: summary.total }));
      setEmail("");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!valid) return;
    if (!window.confirm(t("compliance.deleteConfirm"))) return;
    setLoading(true);
    try {
      const summary = await api.deleteVisitorData(email);
      toast.success(t("compliance.deleteSuccess", { count: summary.total }));
      setEmail("");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <ShieldCheck size={20} />
            {t("compliance.title")}
          </CardTitle>
          <p className="text-sm text-muted-foreground">{t("compliance.description")}</p>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label htmlFor="visitor-email">{t("compliance.visitorEmail")}</Label>
            <Input
              id="visitor-email"
              type="email"
              placeholder={t("compliance.visitorEmailPlaceholder")}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={loading}
            />
            <p className="text-caption text-muted-foreground">{t("compliance.visitorEmailHint")}</p>
          </div>

          <div className="flex flex-wrap gap-3">
            <Button onClick={handleExport} disabled={!valid || loading} variant="outline">
              <Download size={16} className="mr-2" />
              {t("compliance.export")}
            </Button>
            <Button onClick={handleAnonymize} disabled={!valid || loading} variant="secondary">
              <UserMinus size={16} className="mr-2" />
              {t("compliance.anonymize")}
            </Button>
            <Button onClick={handleDelete} disabled={!valid || loading} variant="destructive">
              <Trash size={16} className="mr-2" />
              {t("compliance.delete")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
