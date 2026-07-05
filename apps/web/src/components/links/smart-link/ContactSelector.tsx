import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { CaretDownIcon, CheckIcon, PlusIcon, UserIcon, XIcon } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Combobox } from "@base-ui/react/combobox";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { useTranslation } from "react-i18next";
import type { Contact } from "@/types";

interface ContactSelectorProps {
  workspaceSlug: string;
  value: string[];
  onChange: (contactIds: string[]) => void;
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
  const [comboboxValue, setComboboxValue] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  // Track which contact we're adding so we can clear combobox after add.
  const addTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Cleanup timer on unmount.
  useEffect(() => {
    return () => {
      if (addTimerRef.current) clearTimeout(addTimerRef.current);
    };
  }, []);

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

  const selectedContacts = useMemo(
    () => contacts.filter((c) => value.includes(c.id)),
    [contacts, value],
  );

  const selectedIdSet = useMemo(() => new Set(value), [value]);

  // When contacts finish loading, prune any stale contact IDs from value that
  // no longer exist in the workspace (e.g. restored from draft or edit mode
  // with a contact that has since been deleted). Without this guard the stale
  // IDs would be sent to the backend, causing "contact X not found in
  // workspace" errors.
  const cleanedRef = useRef(false);
  useEffect(() => {
    if (loading || value.length === 0) return;
    if (cleanedRef.current) return;
    const validIds = new Set(contacts.map((c) => c.id));
    const staleIds = value.filter((id) => !validIds.has(id));
    if (staleIds.length > 0) {
      cleanedRef.current = true;
      onChange(value.filter((id) => validIds.has(id)));
    }
  }, [loading, contacts, value, onChange]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const available = contacts.filter((c) => !selectedIdSet.has(c.id));
    if (!q) return available;
    return available.filter(
      (c) =>
        c.email.toLowerCase().includes(q) ||
        c.name.toLowerCase().includes(q) ||
        c.organization?.toLowerCase().includes(q),
    );
  }, [contacts, query, selectedIdSet]);

  const canCreateFromQuery = query.trim() && filtered.length === 0 && isValidEmail(query);

  const goToNewContact = (prefillEmail?: string) => {
    navigate(`/${workspaceSlug}/contacts/new`, {
      state: { from: "link-creator", fromPipeline: true, ...(prefillEmail && { email: prefillEmail }) },
    });
  };

  const addContact = (contactId: string) => {
    if (value.includes(contactId)) return;
    onChange([...value, contactId]);
    // Clear the combobox so it's ready for the next add. Use a microtask to
    // avoid a race with the combobox's own state updates.
    addTimerRef.current = setTimeout(() => {
      setComboboxValue(null);
      setQuery("");
    }, 0);
  };

  const removeContact = (contactId: string) => {
    onChange(value.filter((id) => id !== contactId));
  };

  return (
    <div className="ml-6 space-y-2 rounded-md border border-border p-3">
      <div className="space-y-0.5">
        <Label className="text-sm font-normal">{t("creator.contactLabel")}</Label>
        <p className="text-xs text-muted-foreground">{t("creator.contactHelper")}</p>
      </div>

      {/* Selected contact chips */}
      {selectedContacts.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {selectedContacts.map((contact) => (
            <span
              key={contact.id}
              className="inline-flex items-center gap-1 rounded-full border bg-muted/60 px-2 py-0.5 text-xs"
            >
              <UserIcon size={12} className="text-muted-foreground" />
              <span className="max-w-[160px] truncate">
                {contact.name || contact.email}
              </span>
              <button
                type="button"
                onClick={() => removeContact(contact.id)}
                className="ml-0.5 rounded-full p-0.5 text-muted-foreground hover:bg-muted-foreground/20 hover:text-foreground cursor-pointer"
                aria-label={t("creator.clearContact")}
              >
                <XIcon size={12} />
              </button>
            </span>
          ))}
        </div>
      )}

      <Combobox.Root
        value={comboboxValue}
        onValueChange={(next) => {
          if (next) addContact(next);
        }}
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
            "disabled:cursor-not-allowed disabled:opacity-50",
          )}
        >
          <span className="text-muted-foreground">
            {loading ? t("creator.contactLoading") : t("creator.contactAddMore")}
          </span>
          <Combobox.Icon
            render={<CaretDownIcon size={16} className="shrink-0 text-muted-foreground" />}
          />
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
                "data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95",
              )}
            >
              <div className="p-1">
                <Combobox.Input
                  placeholder={t("creator.contactSearchPlaceholder")}
                  className={cn(
                    "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm outline-none transition-colors",
                    "placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
                  )}
                />
              </div>

              <Combobox.List className="max-h-48 overflow-auto p-1">
                {filtered.map((contact) => (
                  <Combobox.Item
                    key={contact.id}
                    value={contact.id}
                    data-testid={`contact-option-${contact.id}`}
                    className={cn(
                      "group relative flex w-full cursor-default items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none select-none",
                      "hover:bg-accent hover:text-accent-foreground data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground",
                    )}
                  >
                    <UserIcon
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
                      <CheckIcon size={16} weight="bold" />
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
                    <PlusIcon size={14} />
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
                  <PlusIcon size={14} />
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
