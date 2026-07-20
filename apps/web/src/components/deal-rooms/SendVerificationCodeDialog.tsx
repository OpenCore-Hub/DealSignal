import { useEffect, useMemo, useState } from "react";
import { MagnifyingGlass } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import type { AccessRule, Contact, Link } from "@/types";

export interface AllowedVisitor {
  email: string;
  name: string;
}

function ruleValue(rule: AccessRule): string {
  return (
    (rule as { value?: string; Value?: string }).value ??
    (rule as { Value?: string }).Value ??
    ""
  )
    .trim()
    .toLowerCase();
}

function ruleAction(rule: AccessRule): string {
  return (
    (rule as { action?: string; Action?: string }).action ??
    (rule as { Action?: string }).Action ??
    ""
  );
}

/**
 * Build the selectable visitor list strictly from this link's access rules.
 * - Membership = allow emails on the link
 * - Blocked emails are excluded
 * - `contacts` is display-only (name enrichment) and MUST already be link-scoped;
 *   contacts that are not on the allow list never appear in the output.
 */
export function buildAllowedVisitors(
  rules: AccessRule[],
  contacts: Contact[] = [],
): AllowedVisitor[] {
  const blocked = new Set<string>();
  const allowed: string[] = [];
  for (const rule of rules) {
    const value = ruleValue(rule);
    if (!value) continue;
    if (ruleAction(rule) === "block") {
      blocked.add(value);
      continue;
    }
    if (ruleAction(rule) === "allow") {
      allowed.push(value);
    }
  }

  const contactByEmail = new Map(
    contacts.map((c) => [c.email.trim().toLowerCase(), c] as const),
  );

  const seen = new Set<string>();
  const out: AllowedVisitor[] = [];
  for (const email of allowed) {
    if (blocked.has(email) || seen.has(email)) continue;
    seen.add(email);
    const contact = contactByEmail.get(email);
    out.push({
      email,
      name: contact?.name?.trim() || "",
    });
  }
  return out;
}

function extractLinkToken(shortUrl: string): string {
  return shortUrl.split("/").pop() ?? shortUrl;
}

/** Resolve display names only for contacts already attached to this link. */
async function loadLinkScopedContacts(link: Link): Promise<Contact[]> {
  const ids = link.contactIds ?? [];
  if (ids.length === 0) return [];

  const results = await Promise.allSettled(ids.map((id) => api.getContactById(id)));
  return results.flatMap((r) => (r.status === "fulfilled" ? [r.value] : []));
}

interface SendVerificationCodeDialogProps {
  link: Link | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function SendVerificationCodeDialog({
  link,
  open,
  onOpenChange,
}: SendVerificationCodeDialogProps) {
  const { t } = useTranslation("dealRooms");
  const [visitors, setVisitors] = useState<AllowedVisitor[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState("");
  const [loadingList, setLoadingList] = useState(false);
  const [sending, setSending] = useState(false);
  const [loadError, setLoadError] = useState(false);

  useEffect(() => {
    if (!open || !link) {
      setVisitors([]);
      setSelected(new Set());
      setQuery("");
      setLoadError(false);
      return;
    }

    let cancelled = false;
    setLoadingList(true);
    setLoadError(false);

    // Scope: only this link's access rules (+ optional link.contactIds for names).
    // Never load the workspace-wide contact directory.
    Promise.all([api.getLinkAccessRules(link.id), loadLinkScopedContacts(link)])
      .then(([rulesRes, linkContacts]) => {
        if (cancelled) return;
        const next = buildAllowedVisitors(rulesRes.data ?? [], linkContacts);
        setVisitors(next);
        setSelected(new Set(next.map((v) => v.email)));
      })
      .catch(() => {
        if (cancelled) return;
        setLoadError(true);
        setVisitors([]);
        setSelected(new Set());
      })
      .finally(() => {
        if (!cancelled) setLoadingList(false);
      });

    return () => {
      cancelled = true;
    };
  }, [open, link]);

  const filteredVisitors = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return visitors;
    return visitors.filter(
      (v) => v.email.includes(q) || v.name.toLowerCase().includes(q),
    );
  }, [visitors, query]);

  // Mutually exclusive control: either "select all" or "deselect all".
  const allFilteredSelected =
    filteredVisitors.length > 0 && filteredVisitors.every((v) => selected.has(v.email));
  const noneSelected = selected.size === 0;

  const toggleAll = () => {
    if (allFilteredSelected) {
      setSelected((prev) => {
        const next = new Set(prev);
        for (const v of filteredVisitors) next.delete(v.email);
        return next;
      });
      return;
    }
    setSelected((prev) => {
      const next = new Set(prev);
      for (const v of filteredVisitors) next.add(v.email);
      return next;
    });
  };

  const toggleEmail = (email: string, checked: boolean) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(email);
      else next.delete(email);
      return next;
    });
  };

  const handleSend = async () => {
    if (!link || selected.size === 0) return;

    // Defense-in-depth: never send outside the loaded allow-list for this link.
    const allowed = new Set(visitors.map((v) => v.email));
    const emails = [...selected].filter((email) => allowed.has(email));
    if (emails.length === 0) return;

    setSending(true);
    const token = extractLinkToken(link.shortUrl);
    const results = await Promise.allSettled(
      emails.map((email) => api.sendEmailVerificationCode(token, email)),
    );
    const failed = results.filter((r) => r.status === "rejected").length;
    const succeeded = emails.length - failed;
    setSending(false);

    if (succeeded > 0 && failed === 0) {
      toast.success(t("permissions.links.sendCode.successCount", { count: succeeded }));
      onOpenChange(false);
      return;
    }
    if (succeeded > 0 && failed > 0) {
      toast.error(
        t("permissions.links.sendCode.partialError", {
          succeeded,
          failed,
        }),
      );
      return;
    }
    toast.error(t("permissions.links.sendCode.error"));
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("permissions.links.sendCode.title")}</DialogTitle>
          <DialogDescription>
            {t("permissions.links.sendCode.description")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-1">
          {loadingList ? (
            <p className="py-6 text-center text-sm text-muted-foreground">
              {t("common:loading")}
            </p>
          ) : loadError ? (
            <p className="py-6 text-center text-sm text-destructive">
              {t("permissions.links.sendCode.loadError")}
            </p>
          ) : visitors.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">
              {t("permissions.links.sendCode.empty")}
            </p>
          ) : (
            <>
              <div className="flex items-center justify-end gap-3">
                <div className="relative w-28 shrink-0">
                  <MagnifyingGlass
                    size={16}
                    className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-muted-foreground"
                  />
                  <Input
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder={t("permissions.links.sendCode.searchPlaceholder")}
                    disabled={sending}
                    className="h-9 pl-8 focus-visible:border-ring focus-visible:ring-0 focus-visible:ring-offset-0"
                    aria-label={t("permissions.links.sendCode.searchAriaLabel")}
                  />
                </div>
                <label className="flex cursor-pointer items-center gap-2 text-sm whitespace-nowrap text-foreground">
                  <Checkbox
                    checked={allFilteredSelected}
                    onCheckedChange={() => toggleAll()}
                    disabled={sending || filteredVisitors.length === 0}
                    aria-label={
                      allFilteredSelected
                        ? t("permissions.links.sendCode.deselectAll")
                        : t("permissions.links.sendCode.selectAll")
                    }
                  />
                  <span>
                    {allFilteredSelected
                      ? t("permissions.links.sendCode.deselectAll")
                      : t("permissions.links.sendCode.selectAll")}
                  </span>
                </label>
              </div>

              <div className="max-h-60 overflow-y-scroll rounded-lg border [scrollbar-gutter:stable]">
                <table className="w-full text-sm">
                  <thead className="sticky top-0 z-10 bg-muted/80 text-left text-muted-foreground backdrop-blur-sm">
                    <tr>
                      <th className="w-10 px-3 py-2 font-medium" />
                      <th className="px-3 py-2 font-medium">
                        {t("permissions.links.sendCode.contactColumn")}
                      </th>
                      <th className="px-3 py-2 font-medium">
                        {t("permissions.links.sendCode.emailColumn")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredVisitors.length === 0 ? (
                      <tr>
                        <td
                          colSpan={3}
                          className="px-3 py-8 text-center text-sm text-muted-foreground"
                        >
                          {t("permissions.links.sendCode.noSearchResults")}
                        </td>
                      </tr>
                    ) : (
                      filteredVisitors.map((visitor) => {
                        const checked = selected.has(visitor.email);
                        return (
                          <tr
                            key={visitor.email}
                            className="border-t border-border/60 hover:bg-muted/30"
                          >
                            <td className="px-3 py-2.5">
                              <Checkbox
                                checked={checked}
                                disabled={sending}
                                onCheckedChange={(value) =>
                                  toggleEmail(visitor.email, value === true)
                                }
                                aria-label={t("permissions.links.sendCode.selectVisitor", {
                                  email: visitor.email,
                                })}
                              />
                            </td>
                            <td className="px-3 py-2.5 font-medium">
                              {visitor.name || t("permissions.links.sendCode.unnamedContact")}
                            </td>
                            <td className="px-3 py-2.5 text-muted-foreground">
                              {visitor.email}
                            </td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={sending}
          >
            {t("common:cancel")}
          </Button>
          <Button
            type="button"
            disabled={sending || loadingList || loadError || noneSelected}
            onClick={handleSend}
          >
            {sending
              ? t("permissions.links.sendCode.sending")
              : t("permissions.links.sendCode.send")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
