import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router";
import { CaretDown, Check, Plus, User, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Combobox } from "@base-ui/react/combobox";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { useTranslation } from "react-i18next";
import type { Contact } from "@/types";

interface ContactSelectorProps {
  workspaceSlug: string;
  value?: string;
  onChange: (contactId: string | undefined) => void;
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function isValidEmail(value: string) {
  return EMAIL_RE.test(value.trim());
}

export function ContactSelector({ workspaceSlug, value, onChange }: ContactSelectorProps) {
  const { t } = useTranslation("links");
  const navigate = useNavigate();
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState("");

  useEffect(() => {
    let cancelled = false;
    api
      .getContacts()
      .then((res) => {
        if (!cancelled) setContacts(res.data);
      })
      .catch(() => {
        if (!cancelled) setContacts([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const selected = useMemo(() => contacts.find((c) => c.id === value), [contacts, value]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return contacts;
    return contacts.filter(
      (c) =>
        c.email.toLowerCase().includes(q) ||
        c.name.toLowerCase().includes(q) ||
        c.organization?.toLowerCase().includes(q)
    );
  }, [contacts, query]);

  const canCreateFromQuery = query.trim() && filtered.length === 0 && isValidEmail(query);

  const goToNewContact = (prefillEmail?: string) => {
    navigate(`/${workspaceSlug}/contacts/new`, {
      state: { from: "link-creator", ...(prefillEmail && { email: prefillEmail }) },
    });
  };

  return (
    <div className="ml-6 space-y-2 rounded-md border border-border p-3">
      <div className="space-y-0.5">
        <Label className="text-sm font-normal">{t("creator.contactLabel")}</Label>
        <p className="text-xs text-muted-foreground">{t("creator.contactHelper")}</p>
      </div>

      <Combobox.Root
        value={value ?? null}
        onValueChange={(next) => onChange(next ?? undefined)}
        onInputValueChange={setQuery}
        onOpenChange={(open) => {
          if (!open) setQuery("");
        }}
      >
        <Combobox.Trigger
          disabled={loading}
          className={cn(
            "flex w-full items-center justify-between gap-2 rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none transition-colors",
            "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
            "disabled:cursor-not-allowed disabled:opacity-50"
          )}
        >
          {selected ? (
            <div className="flex min-w-0 flex-1 items-center gap-2 overflow-hidden text-left">
              <User size={16} className="shrink-0 text-muted-foreground" />
              <div className="min-w-0">
                <p className="truncate font-medium">{selected.name || selected.email}</p>
                {selected.name && (
                  <p className="truncate text-xs text-muted-foreground">{selected.email}</p>
                )}
              </div>
            </div>
          ) : (
            <span className="text-muted-foreground">
              {loading ? t("creator.contactLoading") : t("creator.contactPlaceholder")}
            </span>
          )}

          {selected ? (
            <span
              role="button"
              tabIndex={-1}
              onClick={(e) => {
                e.stopPropagation();
                onChange(undefined);
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  e.stopPropagation();
                  onChange(undefined);
                }
              }}
              className="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground cursor-pointer"
              aria-label={t("creator.clearContact")}
            >
              <X size={14} />
            </span>
          ) : (
            <Combobox.Icon
              render={<CaretDown size={16} className="shrink-0 text-muted-foreground" />}
            />
          )}
        </Combobox.Trigger>

        <Combobox.Portal>
          <Combobox.Positioner
            className="isolate z-50"
            align="start"
            side="bottom"
            sideOffset={4}
          >
            <Combobox.Popup
              className={cn(
                "z-50 w-[var(--anchor-width)] min-w-36 origin-[var(--transform-origin)] overflow-hidden rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-md outline-none",
                "data-[side=bottom]:slide-in-from-top-2 data-[side=top]:slide-in-from-bottom-2",
                "data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95",
                "data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95"
              )}
            >
              <div className="p-1">
                <Combobox.Input
                  placeholder={t("creator.contactSearchPlaceholder")}
                  className={cn(
                    "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm outline-none transition-colors",
                    "placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                  )}
                />
              </div>

              <Combobox.List className="max-h-48 overflow-auto p-1">
                {filtered.map((contact) => (
                  <Combobox.Item
                    key={contact.id}
                    value={contact.id}
                    className={cn(
                      "group relative flex w-full cursor-default items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none select-none",
                      "hover:bg-accent hover:text-accent-foreground data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground"
                    )}
                  >
                    <User
                      size={16}
                      className="shrink-0 text-muted-foreground group-hover:text-accent-foreground group-data-[highlighted]:text-accent-foreground"
                    />
                    <div className="min-w-0 flex-1">
                      <p className="truncate">{contact.name || contact.email}</p>
                      {contact.name && (
                        <p className="truncate text-xs text-muted-foreground group-hover:text-accent-foreground group-data-[highlighted]:text-accent-foreground">
                          {contact.email}
                        </p>
                      )}
                    </div>
                    <Combobox.ItemIndicator className="ml-auto text-foreground">
                      <Check size={16} weight="bold" />
                    </Combobox.ItemIndicator>
                  </Combobox.Item>
                ))}

                {filtered.length === 0 && !canCreateFromQuery && (
                  <Combobox.Empty className="px-2 py-4 text-center text-sm text-muted-foreground">
                    {query.trim() ? t("creator.contactNoResults") : t("creator.contactEmpty")}
                  </Combobox.Empty>
                )}
              </Combobox.List>

              {canCreateFromQuery && (
                <div className="border-t border-border p-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="w-full justify-start gap-1 text-xs"
                    onClick={() => goToNewContact(query.trim())}
                  >
                    <Plus size={14} />
                    {t("creator.createContactFromSearch", { email: query.trim() })}
                  </Button>
                </div>
              )}

              <div className="border-t border-border p-1">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="w-full justify-start gap-1 text-xs"
                  onClick={() => goToNewContact()}
                >
                  <Plus size={14} />
                  {t("creator.newContact")}
                </Button>
              </div>
            </Combobox.Popup>
          </Combobox.Positioner>
        </Combobox.Portal>
      </Combobox.Root>
    </div>
  );
}
