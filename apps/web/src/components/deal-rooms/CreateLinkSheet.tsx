import { useState } from "react";
import { Eye, CaretDown, Question, Tag } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
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
  children: React.ReactElement;
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

export function CreateLinkSheet({ children }: CreateLinkSheetProps) {
  const { t } = useTranslation("dealRooms");
  const [open, setOpen] = useState(false);
  const [linkName, setLinkName] = useState("");
  const [domain, setDomain] = useState(domains[0]);
  const [preset, setPreset] = useState("");
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [options, setOptions] = useState(defaultOptions);
  const [expanded, setExpanded] = useState({
    securityControls: true,
    advancedControls: true,
  });

  const toggle = (key: OptionKey) => {
    setOptions((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={children} />
      <DialogContent
        showCloseButton
        className="fixed inset-y-0 right-0 top-0 left-auto flex h-full w-full max-w-md flex-col translate-x-0 translate-y-0 rounded-none rounded-l-xl border-y-0 border-r-0 border-l border-border bg-popover p-6 shadow-xl sm:max-w-md data-open:animate-in data-open:slide-in-from-right data-closed:animate-out data-closed:slide-out-to-right"
      >
        <DialogHeader>
          <DialogTitle>{t("permissions.createLinkSheet.title")}</DialogTitle>
        </DialogHeader>

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
              <Button variant="link" size="sm" className="h-auto p-0">
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
              <OptionSwitch
                key={key}
                label={t(`permissions.createLinkSheet.${key}`)}
                checked={options[key]}
                onCheckedChange={() => toggle(key)}
              />
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
              <Button variant="link" size="sm" className="h-auto p-0">
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
            <Button variant="outline">{t("permissions.createLinkSheet.saveLink")}</Button>
            <Button>{t("permissions.createLinkSheet.manageFilePermissions")}</Button>
          </div>
        </div>
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
