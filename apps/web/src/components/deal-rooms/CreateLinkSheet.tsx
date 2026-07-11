import { useState } from "react";
import { Eye, CaretDown, Question, Tag, Check, Copy } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { api, type CreateDealRoomLinkPayload } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";
import type { Link } from "@/types";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

interface CreateLinkSheetProps {
  dealRoomId: string;
  children: React.ReactElement;
  onCreated?: (link: Link) => void;
}

const domains = ["papermark.com", "invest.acme.capital"];
const presets = ["public", "standard", "confidential"];
const tags = ["investors", "lp-report", "q2-2026"];

const linkOptionKeys = [
  "receiveEmailNotification",
  "requireEmailToView",
  "requireEmailVerification",
  "allowDownloading",
  "allowSpecifiedViewers",
  "blockSpecifiedViewers",
] as const;

const securityControlKeys = [
  "requirePassword",
  "expirationDate",
  "enableScreenshotProtection",
  "applyWatermark",
  "requireNda",
  "customFormFields",
] as const;

const advancedControlKeys = [
  "aiAgents",
  "enableFileRequests",
  "enableIndexFileGeneration",
  "enableQaConversations",
] as const;

type OptionKey =
  | (typeof linkOptionKeys)[number]
  | (typeof securityControlKeys)[number]
  | (typeof advancedControlKeys)[number];

const defaultOptions: Record<OptionKey, boolean> = {
  receiveEmailNotification: true,
  requireEmailToView: true,
  requireEmailVerification: false,
  allowDownloading: false,
  allowSpecifiedViewers: false,
  blockSpecifiedViewers: false,
  requirePassword: false,
  expirationDate: false,
  enableScreenshotProtection: false,
  applyWatermark: false,
  requireNda: false,
  customFormFields: false,
  aiAgents: false,
  enableFileRequests: false,
  enableIndexFileGeneration: false,
  enableQaConversations: false,
};

export function CreateLinkSheet({ dealRoomId, children, onCreated }: CreateLinkSheetProps) {
  const { t } = useTranslation("dealRooms");
  const [open, setOpen] = useState(false);
  const [linkName, setLinkName] = useState("");
  const [domain, setDomain] = useState(domains[0]);
  const [preset, setPreset] = useState("");
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [options, setOptions] = useState(defaultOptions);
  const [password, setPassword] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [expanded, setExpanded] = useState({
    securityControls: true,
    advancedControls: true,
  });
  const [saving, setSaving] = useState(false);
  const [createdLink, setCreatedLink] = useState<Link | null>(null);
  const [copied, setCopied] = useState(false);

  const toggle = (key: OptionKey) => {
    setOptions((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const handleSave = async () => {
    if (!linkName.trim()) {
      toast.error(t("permissions.createLinkSheet.linkNameRequired"));
      return;
    }
    if (options.requirePassword && !password) {
      toast.error(t("permissions.createLinkSheet.passwordRequired"));
      return;
    }

    const payload: CreateDealRoomLinkPayload = {
      name: linkName.trim(),
      require_email: options.requireEmailToView,
      require_email_verification: options.requireEmailVerification,
      require_nda: options.requireNda,
      require_password: options.requirePassword,
      password: options.requirePassword ? password : undefined,
      expires_at: options.expirationDate && expiresAt ? new Date(expiresAt).toISOString() : undefined,
      download_enabled: options.allowDownloading,
      watermark_enabled: options.applyWatermark,
      ai_copilot_enabled: options.aiAgents,
      custom_domain: domain,
      tags: selectedTags.length > 0 ? selectedTags : [],
      notify_on_access: options.receiveEmailNotification,
    };

    setSaving(true);
    try {
      const link = await api.createDealRoomLink(dealRoomId, payload);
      setCreatedLink(link);
      onCreated?.(link);
      toast.success(t("permissions.createLinkSheet.created"));
    } catch {
      toast.error(t("permissions.createLinkSheet.createError"));
    } finally {
      setSaving(false);
    }
  };

  const handleCopy = async () => {
    if (!displayUrl) return;
    const ok = await copyToClipboard(displayUrl, t("permissions.createLinkSheet.copied"));
    if (ok) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleClose = () => {
    setOpen(false);
    // Reset form after animation.
    setTimeout(() => {
      setLinkName("");
      setPreset("");
      setSelectedTags([]);
      setOptions(defaultOptions);
      setPassword("");
      setExpiresAt("");
      setCreatedLink(null);
    }, 200);
  };

  const displayUrl = createdLink?.shortUrl ?? "";

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={children} />
      <DialogContent
        showCloseButton
        className="fixed inset-y-0 right-0 top-0 left-auto flex h-full w-full max-w-md flex-col translate-x-0 translate-y-0 rounded-none rounded-l-xl border-y-0 border-r-0 border-l border-border bg-popover p-6 shadow-xl sm:max-w-md data-open:animate-in data-open:slide-in-from-right data-closed:animate-out data-closed:slide-out-to-right"
      >
        <DialogHeader>
          <DialogTitle>
            {createdLink
              ? t("permissions.createLinkSheet.createdTitle")
              : t("permissions.createLinkSheet.title")}
          </DialogTitle>
        </DialogHeader>

        {createdLink ? (
          <div className="flex flex-1 flex-col gap-6 py-4">
            <div className="rounded-lg border border-success-500/20 bg-success-500/10 p-4">
              <div className="flex items-center gap-2 text-sm font-medium text-success-600">
                <Check size={18} />
                {t("permissions.createLinkSheet.created")}
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                {t("permissions.createLinkSheet.createdHint")}
              </p>
            </div>

            <div className="space-y-2">
              <Label>{t("permissions.createLinkSheet.shortUrl")}</Label>
              <div className="flex items-center gap-2">
                <Input readOnly value={displayUrl} className="flex-1" />
                <Button
                  variant="outline"
                  size="icon"
                  onClick={handleCopy}
                  aria-label={t("permissions.createLinkSheet.copyLink")}
                >
                  {copied ? <Check size={16} className="text-success-500" /> : <Copy size={16} />}
                </Button>
              </div>
            </div>

            <div className="mt-auto flex justify-end gap-2 pt-6">
              <Button onClick={handleClose}>{t("common:close")}</Button>
            </div>
          </div>
        ) : (
          <>
            <div className="flex-1 space-y-6 overflow-y-auto py-2 pr-1">
              <div className="space-y-2">
                <Label>{t("permissions.createLinkSheet.linkName")}</Label>
                <Input
                  value={linkName}
                  onChange={(e) => setLinkName(e.target.value)}
                  placeholder={t("permissions.createLinkSheet.linkNamePlaceholder")}
                />
              </div>

              <div className="space-y-2">
                <Label>{t("permissions.createLinkSheet.domain")}</Label>
                <Select value={domain} onValueChange={(v) => setDomain(v ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {domains.map((d) => (
                      <SelectItem key={d} value={d}>
                        {d}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label>{t("permissions.createLinkSheet.linkPreset")}</Label>
                  <Button
                    variant="link"
                    size="sm"
                    className="h-auto p-0"
                    disabled
                    title={t("permissions.createLinkSheet.manageDisabled")}
                  >
                    {t("permissions.createLinkSheet.manage")}
                  </Button>
                </div>
                <Select value={preset} onValueChange={(v) => setPreset(v ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t("permissions.createLinkSheet.selectPreset")} />
                  </SelectTrigger>
                  <SelectContent>
                    {presets.map((p) => (
                      <SelectItem key={p} value={p}>
                        {t(`permissions.createLinkSheet.presets.${p}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-caption text-muted-foreground">
                  {t("permissions.createLinkSheet.presetHint")}
                </p>
              </div>

              <div className="relative flex items-center py-2">
                <div className="flex-1 border-t" />
                <span className="mx-3 text-xs text-muted-foreground">
                  {t("permissions.createLinkSheet.linkOptions")}
                </span>
                <div className="flex-1 border-t" />
              </div>

              <div className="space-y-4">
                {linkOptionKeys.map((key) => (
                  <OptionSwitch
                    key={key}
                    label={t(`permissions.createLinkSheet.${key}`)}
                    checked={options[key]}
                    onCheckedChange={() => toggle(key)}
                  />
                ))}
              </div>

              <CollapsibleSection
                title={t("permissions.createLinkSheet.securityControls")}
                open={expanded.securityControls}
                onToggle={() => setExpanded((s) => ({ ...s, securityControls: !s.securityControls }))}
              >
                {securityControlKeys.map((key) => (
                  <div key={key} className="space-y-2">
                    <OptionSwitch
                      label={t(`permissions.createLinkSheet.${key}`)}
                      checked={options[key]}
                      onCheckedChange={() => toggle(key)}
                    />
                    {key === "requirePassword" && options.requirePassword && (
                      <Input
                        type="password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        placeholder={t("permissions.createLinkSheet.passwordPlaceholder")}
                      />
                    )}
                    {key === "expirationDate" && options.expirationDate && (
                      <Input
                        type="datetime-local"
                        value={expiresAt}
                        onChange={(e) => setExpiresAt(e.target.value)}
                      />
                    )}
                  </div>
                ))}
              </CollapsibleSection>

              <CollapsibleSection
                title={t("permissions.createLinkSheet.advancedControls")}
                open={expanded.advancedControls}
                onToggle={() => setExpanded((s) => ({ ...s, advancedControls: !s.advancedControls }))}
              >
                {advancedControlKeys.map((key) => (
                  <OptionSwitch
                    key={key}
                    label={t(`permissions.createLinkSheet.${key}`)}
                    checked={options[key]}
                    onCheckedChange={() => toggle(key)}
                  />
                ))}
              </CollapsibleSection>

              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label className="flex items-center gap-1.5">
                    {t("permissions.createLinkSheet.tags")}
                    <Question size={14} className="text-muted-foreground" />
                  </Label>
                  <Button
                    variant="link"
                    size="sm"
                    className="h-auto p-0"
                    disabled
                    title={t("permissions.createLinkSheet.manageDisabled")}
                  >
                    {t("permissions.createLinkSheet.manage")}
                  </Button>
                </div>
                <Select
                  value={selectedTags[0] ?? ""}
                  onValueChange={(v) => setSelectedTags(v ? [v] : [])}
                >
                  <SelectTrigger className="w-full">
                    <Tag size={16} className="text-muted-foreground" />
                    <SelectValue placeholder={t("permissions.createLinkSheet.selectTags")} />
                  </SelectTrigger>
                  <SelectContent>
                    {tags.map((tag) => (
                      <SelectItem key={tag} value={tag}>
                        {tag}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="flex items-center justify-between gap-3 pt-6">
              <Button variant="ghost" size="icon" aria-label={t("permissions.createLinkSheet.preview")}>
                <Eye size={20} />
              </Button>
              <div className="flex items-center gap-2">
                <Button variant="outline" onClick={() => setOpen(false)}>
                  {t("common:cancel")}
                </Button>
                <Button onClick={handleSave} disabled={saving}>
                  {saving ? t("common:saving") : t("permissions.createLinkSheet.saveLink")}
                </Button>
              </div>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

function CollapsibleSection({
  title,
  open,
  onToggle,
  children,
}: {
  title: string;
  open: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="shrink-0">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center justify-between py-3 text-sm font-medium"
      >
        {title}
        <CaretDown size={16} className={cn("transition-transform", open && "rotate-180")} />
      </button>
      {open && <div className="space-y-3 pb-2">{children}</div>}
    </div>
  );
}

function OptionSwitch({
  label,
  checked,
  onCheckedChange,
}: {
  label: string;
  checked: boolean;
  onCheckedChange: () => void;
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <Label className="flex items-center gap-1.5 font-normal text-foreground">
        {label}
        <Question size={14} className="text-muted-foreground" />
      </Label>
      <Switch checked={checked} onCheckedChange={onCheckedChange} />
    </div>
  );
}
