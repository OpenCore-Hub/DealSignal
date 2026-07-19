import { useState, useRef } from "react";
import { Envelope, Minus, Plus, UserPlus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { DealRoomMemberRole } from "@/types";

interface InviteMemberDialogProps {
  roomId: string;
  onInvited: () => void;
  children?: React.ReactNode;
}

const isValidEmail = (email: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);

export function InviteMemberDialog({ roomId, onInvited, children }: InviteMemberDialogProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const [open, setOpen] = useState(false);
  const [emails, setEmails] = useState<string[]>([""]);
  const [role, setRole] = useState<DealRoomMemberRole>("viewer");
  const [submitting, setSubmitting] = useState(false);
  const [showErrors, setShowErrors] = useState(false);
  const [touched, setTouched] = useState<Set<number>>(new Set());
  const listRef = useRef<HTMLDivElement>(null);

  const handleEmailChange = (index: number, value: string) => {
    const next = [...emails];
    next[index] = value;
    setEmails(next);
    setShowErrors(false);
  };

  const addEmailField = (index: number) => {
    const next = [...emails];
    next.splice(index + 1, 0, "");
    setEmails(next);
  };

  const removeEmailField = (index: number) => {
    if (emails.length <= 1) return;
    const next = [...emails];
    next.splice(index, 1);
    setEmails(next);
    setShowErrors(false);
    setTouched(new Set());
  };

  const handleKeyDown = (index: number, e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      addEmailField(index);
      // Focus the newly added input on the next render.
      setTimeout(() => {
        const inputs = listRef.current?.querySelectorAll("input[type='email']");
        const nextInput = inputs?.[index + 1] as HTMLInputElement | undefined;
        nextInput?.focus();
      }, 0);
    }
  };

  const handleInvite = async () => {
    const trimmedEmails = emails.map((e) => e.trim()).filter(Boolean);
    if (trimmedEmails.length === 0) return;

    const invalid = trimmedEmails.filter((email) => !isValidEmail(email));
    if (invalid.length > 0) {
      setShowErrors(true);
      toast.error(t("members.invalidEmails", { emails: invalid.join(", ") }));
      return;
    }

    setSubmitting(true);
    try {
      const results = await Promise.allSettled(
        trimmedEmails.map((email) => api.inviteDealRoomMember(roomId, { email, role }))
      );

      const succeeded = results.filter((r) => r.status === "fulfilled").length;
      const failed = results.filter((r) => r.status === "rejected").length;

      if (failed === 0) {
        if (succeeded === 1) {
          toast.success(t("members.invited", { email: trimmedEmails[0] }));
        } else {
          toast.success(t("members.invitedCount", { count: succeeded }));
        }
        setEmails([""]);
        setRole("viewer");
        setOpen(false);
        onInvited();
      } else {
        toast.error(
          t("members.invitedSomeFailed", {
            invited: succeeded,
            total: trimmedEmails.length,
            failed,
          })
        );
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setSubmitting(false);
    }
  };

  const hasValidEmail = emails.some((email) => isValidEmail(email.trim()));

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={children ? (children as React.ReactElement) : (
        <Button variant="outline" className="gap-1.5">
          <Envelope size={16} />
          {t("detail.invite")}
        </Button>
      )} />
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <UserPlus size={20} />
            {t("members.inviteTitle")}
          </DialogTitle>
          <DialogDescription>{t("members.inviteDescription")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="member-email-0">{t("members.email")}</Label>
            </div>
            <div
              ref={listRef}
              className="max-h-[480px] overflow-y-auto pr-1 space-y-2"
            >
              {emails.map((email, index) => {
                const isLast = index === emails.length - 1;
                const isInvalid =
                  (showErrors || touched.has(index)) &&
                  email.trim() !== "" &&
                  !isValidEmail(email.trim());
                return (
                  <div key={index} className="flex items-start gap-2">
                    <div className="flex-1 space-y-1">
                      <Input
                        id={index === 0 ? "member-email-0" : undefined}
                        type="email"
                        value={email}
                        onChange={(e) => handleEmailChange(index, e.target.value)}
                        onBlur={() =>
                          setTouched((prev) => new Set(prev).add(index))
                        }
                        onKeyDown={(e) => handleKeyDown(index, e)}
                        placeholder={t("members.emailPlaceholder")}
                        aria-label={t("members.email")}
                        aria-invalid={isInvalid}
                        aria-describedby={
                          isInvalid ? `email-error-${index}` : undefined
                        }
                        className={cn(
                          "flex-1",
                          isInvalid &&
                            "border-destructive focus-visible:ring-destructive"
                        )}
                      />
                      {isInvalid && (
                        <p
                          id={`email-error-${index}`}
                          className="text-xs text-destructive"
                        >
                          {t("members.emailInvalid")}
                        </p>
                      )}
                    </div>
                    {isLast ? (
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 shrink-0 rounded-full text-muted-foreground hover:bg-primary/10 hover:text-primary"
                        onClick={() => addEmailField(index)}
                        aria-label={t("members.addEmail")}
                        title={t("members.addEmail")}
                      >
                        <Plus size={14} />
                      </Button>
                    ) : (
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 shrink-0 rounded-full text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                        onClick={() => removeEmailField(index)}
                        aria-label={t("members.removeEmail")}
                        title={t("members.removeEmail")}
                      >
                        <Minus size={14} />
                      </Button>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="member-role">{t("members.role")}</Label>
            <Select value={role} onValueChange={(v) => setRole(v as DealRoomMemberRole)}>
              <SelectTrigger id="member-role" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent
                side="bottom"
                align="start"
                alignItemWithTrigger={false}
                collisionAvoidance={{ side: "none", align: "none" }}
              >
                <SelectItem value="viewer" label={t("members.roles.viewer")}>
                  {t("members.roles.viewer")}
                </SelectItem>
                <SelectItem value="member" label={t("members.roles.member")}>
                  {t("members.roles.member")}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => setOpen(false)}>
            {tc("cancel")}
          </Button>
          <Button type="button" onClick={handleInvite} disabled={!hasValidEmail || submitting}>
            {submitting ? t("members.inviting") : t("detail.invite")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
