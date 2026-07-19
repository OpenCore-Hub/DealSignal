import {
  useState,
  useRef,
  useEffect,
  useMemo,
  useCallback,
  type KeyboardEvent,
} from "react";
import { useTranslation } from "react-i18next";
import {
  X,
  UserPlus,
  CaretDown,
  MagnifyingGlass,
  User,
  Users,
} from "@phosphor-icons/react";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { Contact } from "@/types";

interface ContactEmailTagInputProps {
  id?: string;
  values: string[];
  onChange: (values: string[]) => void;
  placeholder?: string;
  hint?: string;
  disabled?: boolean;
  conflictValues?: string[];
  allowDomains?: boolean;
}

function normalize(value: string, allowDomain: boolean): string {
  const trimmed = value.trim().toLowerCase();
  if (allowDomain && trimmed.startsWith("@")) {
    return `*${trimmed}`;
  }
  return trimmed;
}

function isValid(value: string, allowDomain: boolean): boolean {
  if (allowDomain && value.startsWith("*@") && value.length > 2) {
    const domain = value.slice(2);
    return domain.includes(".") && !domain.includes("@");
  }
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
}

const SEPARATORS = [",", ";", " ", "\n", "\t"];

export function ContactEmailTagInput({
  id,
  values,
  onChange,
  placeholder,
  hint,
  disabled,
  conflictValues = [],
  allowDomains = false,
}: ContactEmailTagInputProps) {
  const { t } = useTranslation("linkShare");
  const { t: tc } = useTranslation("common");
  const [raw, setRaw] = useState("");
  const [invalid, setInvalid] = useState<string[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const [contacts, setContacts] = useState<Contact[]>([]);
  const [contactsLoading, setContactsLoading] = useState(true);

  const [contactListOpen, setContactListOpen] = useState(false);
  const [contactSearch, setContactSearch] = useState("");

  const [addContactOpen, setAddContactOpen] = useState(false);
  const [newContactEmail, setNewContactEmail] = useState("");
  const [newContactName, setNewContactName] = useState("");
  const [creatingContact, setCreatingContact] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const res = await api.getContacts();
        if (!cancelled) setContacts(res.data);
      } catch {
        if (!cancelled) setContacts([]);
      } finally {
        if (!cancelled) setContactsLoading(false);
      }
    };
    load();
    return () => {
      cancelled = true;
    };
  }, []);

  const contactByEmail = useMemo(() => {
    const map = new Map<string, Contact>();
    for (const contact of contacts) {
      map.set(contact.email.toLowerCase(), contact);
    }
    return map;
  }, [contacts]);

  const displayValue = useCallback(
    (value: string | undefined) => {
      if (!value || typeof value !== "string") return "";
      const contact = contactByEmail.get(value.toLowerCase());
      return contact?.name || value;
    },
    [contactByEmail]
  );

  const autoResize = useCallback(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.max(el.scrollHeight, 22)}px`;
  }, []);

  useEffect(() => {
    autoResize();
  }, [raw, autoResize]);

  const commit = (input: string) => {
    const parts = input
      .split(new RegExp(`[${SEPARATORS.map((s) => `\\${s}`).join("")}]`))
      .map((v) => normalize(v, allowDomains))
      .filter(Boolean);

    const next = [...values];
    const nextInvalid: string[] = [];

    for (const part of parts) {
      if (!isValid(part, allowDomains)) {
        nextInvalid.push(part);
        continue;
      }
      if (!next.includes(part)) {
        next.push(part);
      }
    }

    const changed =
      next.length !== values.length || next.some((v, i) => v !== values[i]);
    if (changed) {
      onChange(next);
    }
    setRaw(nextInvalid.join(", "));
    setInvalid(nextInvalid);
  };

  const addValue = (value: string) => {
    const normalized = normalize(value, allowDomains);
    if (!normalized || values.includes(normalized) || !isValid(normalized, allowDomains)) {
      return;
    }
    onChange([...values, normalized]);
  };

  const remove = (value: string) => {
    onChange(values.filter((v) => v !== value));
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      commit(raw);
      return;
    }
    if (e.key === "Backspace" && raw === "" && values.length > 0) {
      e.preventDefault();
      remove(values[values.length - 1]);
    }
  };

  const handleBlur = () => {
    if (raw.trim()) {
      commit(raw);
    }
  };

  const handleSelectContact = (contact: Contact) => {
    addValue(contact.email);
    setContactListOpen(false);
    setContactSearch("");
  };

  const handleCreateContact = async () => {
    if (!newContactEmail) return;
    setCreatingContact(true);
    try {
      const contact = await api.createContact({
        email: newContactEmail,
        name: newContactName,
      });
      setContacts((prev) => [...prev, contact]);
      addValue(contact.email);
      setAddContactOpen(false);
      setNewContactEmail("");
      setNewContactName("");
    } catch {
      // Error toast is handled by the api client.
    } finally {
      setCreatingContact(false);
    }
  };

  const filteredContacts = useMemo(() => {
    const q = contactSearch.trim().toLowerCase();
    const selected = new Set(
      values.filter((v) => v != null && typeof v === "string").map((v) => v.toLowerCase())
    );
    const available = contacts.filter(
      (c) => c.email && !selected.has(c.email.toLowerCase())
    );
    if (!q) return available;
    return available.filter(
      (c) =>
        c.name?.toLowerCase().includes(q) ||
        c.email?.toLowerCase().includes(q) ||
        c.organization?.toLowerCase().includes(q)
    );
  }, [contacts, contactSearch, values]);

  const canCreateContact =
    newContactEmail && isValid(newContactEmail.toLowerCase(), allowDomains);

  return (
    <div className="w-full space-y-2">
      <div
        className={cn(
          "flex max-w-full min-h-[80px] flex-wrap items-start gap-2 overflow-hidden rounded-md border border-input bg-background p-2 focus-within:ring-1 focus-within:ring-ring",
          disabled && "cursor-not-allowed opacity-50",
          invalid.length > 0 &&
            "border-destructive focus-within:ring-destructive"
        )}
        onClick={() => textareaRef.current?.focus()}
      >
        {values
          .filter((v): v is string => typeof v === "string" && v.length > 0)
          .map((value) => {
          const label = displayValue(value);
          const isConflict = conflictValues.includes(value);
          return (
            <span
              key={value}
              className={cn(
                "inline-flex max-w-full animate-in fade-in zoom-in items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium duration-200",
                isConflict
                  ? "border border-destructive/30 bg-destructive/10 text-destructive"
                  : "bg-primary/10 text-primary"
              )}
              title={value}
            >
              <span className="truncate">{label}</span>
              {!disabled && (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    remove(value);
                  }}
                  className="shrink-0 rounded-full p-0.5 hover:bg-primary/20"
                  aria-label={t("emailTagInput.remove", { value: label })}
                >
                  <X size={12} />
                </button>
              )}
            </span>
          );
        })}
        <div className="flex min-w-[120px] flex-1 items-start gap-1">
          <textarea
            id={id}
            ref={textareaRef}
            value={raw}
            rows={1}
            onChange={(e) => setRaw(e.target.value)}
            onKeyDown={handleKeyDown}
            onBlur={handleBlur}
            placeholder={values.length === 0 ? placeholder : ""}
            disabled={disabled}
            className="h-auto min-h-[22px] min-w-0 flex-1 resize-none border-0 bg-transparent px-1 py-0.5 text-sm shadow-none outline-none focus-visible:ring-0"
          />
          <DropdownMenu>
            <DropdownMenuTrigger
              className="mt-0.5 inline-flex shrink-0 items-center gap-1 rounded-md px-1.5 py-1 text-xs text-muted-foreground outline-none hover:bg-muted hover:text-foreground disabled:opacity-50"
              aria-label={t("contactPicker.menuLabel")}
              onClick={(e) => e.stopPropagation()}
              onMouseDown={(e) => e.preventDefault()}
              disabled={disabled}
            >
              <UserPlus size={14} />
              <CaretDown size={14} />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-44">
            <DropdownMenuItem
              onClick={() => setContactListOpen(true)}
              className="gap-2"
            >
              <Users size={16} />
              {t("contactPicker.contactList")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => setAddContactOpen(true)}
              className="gap-2"
            >
              <UserPlus size={16} />
              {t("contactPicker.addContact")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      </div>
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      {invalid.length > 0 && (
        <p className="text-xs text-destructive">
          {invalid.join(", ")} — {t("emailTagInput.invalid")}
        </p>
      )}

      {/* Contact list dialog */}
      <Dialog open={contactListOpen} onOpenChange={setContactListOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("contactPicker.contactListTitle")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="relative">
              <MagnifyingGlass
                size={16}
                className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                value={contactSearch}
                onChange={(e) => setContactSearch(e.target.value)}
                placeholder={t("contactPicker.searchContacts")}
                className="pl-9"
              />
            </div>
            <div className="max-h-64 space-y-1 overflow-y-auto">
              {contactsLoading ? (
                <p className="py-6 text-center text-sm text-muted-foreground">
                  {t("contactPicker.loading")}
                </p>
              ) : filteredContacts.length === 0 ? (
                <p className="py-6 text-center text-sm text-muted-foreground">
                  {contactSearch.trim()
                    ? t("contactPicker.noSearchResults")
                    : t("contactPicker.noContacts")}
                </p>
              ) : (
                filteredContacts.map((contact) => (
                  <button
                    key={contact.id}
                    type="button"
                    onClick={() => handleSelectContact(contact)}
                    className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-accent hover:text-accent-foreground"
                  >
                    <User size={16} className="shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <p className="truncate">
                        {contact.name || contact.email}
                      </p>
                      {contact.name && (
                        <p className="truncate text-xs text-muted-foreground">
                          {contact.email}
                        </p>
                      )}
                    </div>
                  </button>
                ))
              )}
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setContactListOpen(false)}
            >
              {tc("cancel")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Add contact dialog */}
      <Dialog open={addContactOpen} onOpenChange={setAddContactOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("contactPicker.addContactTitle")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="new-contact-email">
                {t("contactPicker.email")}
              </Label>
              <Input
                id="new-contact-email"
                type="email"
                value={newContactEmail}
                onChange={(e) => setNewContactEmail(e.target.value)}
                placeholder={t("contactPicker.emailPlaceholder")}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-contact-name">
                {t("contactPicker.name")}
              </Label>
              <Input
                id="new-contact-name"
                value={newContactName}
                onChange={(e) => setNewContactName(e.target.value)}
                placeholder={t("contactPicker.namePlaceholder")}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setAddContactOpen(false)}
            >
              {tc("cancel")}
            </Button>
            <Button
              onClick={handleCreateContact}
              disabled={creatingContact || !canCreateContact}
            >
              {creatingContact
                ? t("contactPicker.creating")
                : t("contactPicker.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
