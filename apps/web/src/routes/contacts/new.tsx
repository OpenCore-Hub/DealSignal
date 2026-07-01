import { useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { motion } from "motion/react";
import { UserPlus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";
import { BackButton } from "@/components/common/BackButton";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";

export function NewContactPage() {
  const { t } = useTranslation("contacts");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const location = useLocation();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const reducedMotion = useReducedMotion();
  const state = location.state as { from?: string; email?: string } | null;
  const fromLinkCreator = state?.from === "link-creator";

  const [name, setName] = useState("");
  const [email, setEmail] = useState(() => state?.email ?? "");
  const [creating, setCreating] = useState(false);

  const handleCreate = async () => {
    if (!email) {
      toast.error(t("new.emailRequired"));
      return;
    }
    setCreating(true);
    try {
      const contact = await api.createContact({ email, name });
      toast.success(t("new.created"));
      if (fromLinkCreator) {
        navigate(`/${workspaceSlug}/links/new`, {
          state: { newContactId: contact.id },
        });
      } else {
        navigate(`/${workspaceSlug}/contacts`);
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("new.createFailed"));
    } finally {
      setCreating(false);
    }
  };

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      className="mx-auto max-w-2xl space-y-6"
    >
      <BackButton
        to={fromLinkCreator ? `/${workspaceSlug}/links/new` : `/${workspaceSlug}/contacts`}
        label={tc("back")}
      />

      <div className="space-y-1">
        <h1 className="text-h1">{t("new.title")}</h1>
        <p className="text-body text-muted-foreground">{t("new.subtitle")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <UserPlus size={20} />
            {t("new.cardTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="contact-email">{t("new.email")}</Label>
            <Input
              id="contact-email"
              type="email"
              placeholder={t("new.emailPlaceholder")}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="contact-name">{t("new.name")}</Label>
            <Input
              id="contact-name"
              placeholder={t("new.namePlaceholder")}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <Button
              variant="outline"
              onClick={() =>
                navigate(fromLinkCreator ? `/${workspaceSlug}/links/new` : `/${workspaceSlug}/contacts`)
              }
            >
              {tc("cancel")}
            </Button>
            <Button onClick={handleCreate} disabled={creating || !email}>
              {creating ? tc("creating") : t("new.create")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </motion.div>
  );
}

export default NewContactPage;
