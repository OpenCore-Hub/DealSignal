import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Check } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { ApiError } from "@/lib/apiClient";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { ContactEmailTagInput } from "@/components/links/share";
import { DealRoomAccessRequestsPanel } from "./DealRoomAccessRequestsPanel";
import {
  draftFromRoomAccessPolicy,
  roomAccessPolicyPayloadFromDraft,
} from "./roomAccessPolicy";
import type { DraftLink } from "@/components/links/share";

interface DealRoomAccessControlTabProps {
  roomId: string;
  /** @deprecated Room policy is room-scoped; kept for call-site compatibility. */
  initialLinkId?: string;
  /** Highlight a pending share-link applicant from a dashboard deep link. */
  focusLinkId?: string;
  onChanged?: () => void | Promise<void>;
  /** Notifies parent when local edits are unsaved (for tab-leave guard). */
  onDirtyChange?: (dirty: boolean) => void;
  canManage?: boolean;
}

export function DealRoomAccessControlTab({
  roomId,
  focusLinkId,
  onChanged,
  onDirtyChange,
  canManage = false,
}: DealRoomAccessControlTabProps) {
  const { t } = useTranslation("dealRooms");
  const { t: lt } = useTranslation("linkShare");
  const { t: tc } = useTranslation("common");

  const [draft, setDraft] = useState<DraftLink>(() => draftFromRoomAccessPolicy(null));
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const dirtyRef = useRef(false);

  const { data, loading, refetch } = useAsyncData(
    () => api.getDealRoomAccessPolicy(roomId).then((res) => res.data),
    [roomId],
  );
  const { data: billing } = useAsyncData(
    () => (canManage ? api.getBillingInfo().catch(() => null) : Promise.resolve(null)),
    [canManage],
  );

  const setDirtyState = useCallback(
    (next: boolean) => {
      if (dirtyRef.current === next) return;
      dirtyRef.current = next;
      onDirtyChange?.(next);
    },
    [onDirtyChange],
  );

  useEffect(() => {
    onDirtyChange?.(dirtyRef.current);
    return () => {
      onDirtyChange?.(false);
    };
  }, [onDirtyChange]);

  const loadedKeyRef = useRef<string | undefined>(undefined);
  useLayoutEffect(() => {
    if (!data) return;
    const currentKey = `policy:${roomId}:${data.updatedAt ?? data.configured}`;
    const keyChanged = currentKey !== loadedKeyRef.current;
    if (keyChanged && dirtyRef.current) {
      // Keep loadedKeyRef so save+refetch (or dirty clear) still applies server.
      return;
    }
    if (keyChanged || !dirtyRef.current) {
      setDraft(draftFromRoomAccessPolicy(data));
      setDirtyState(false);
      loadedKeyRef.current = currentKey;
    }
  }, [data, roomId, setDirtyState]);

  const updateDraft = useCallback(
    (patch: Partial<DraftLink>) => {
      setDraft((prev) => ({ ...prev, ...patch }));
      setDirtyState(true);
    },
    [setDirtyState],
  );

  const verifyEnableBlocked =
    billing != null && billing.accessControlsEnabled === false && !draft.requireEmailVerification;
  const blocklistEnableBlocked =
    billing != null &&
    billing.accessControlsEnabled === false &&
    draft.blockedViewers.length === 0;
  const ndaEnableBlocked = billing != null && billing.ndaEnabled === false && !draft.requireNda;

  const handleSave = async () => {
    if (!canManage) return;
    setSaving(true);
    try {
      await api.upsertDealRoomAccessPolicy(roomId, roomAccessPolicyPayloadFromDraft(draft));
      setDirtyState(false);
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 1500);
      toast.success(lt("accessRules.saved"));
      await refetch();
      await onChanged?.();
    } catch (err) {
      if (err instanceof ApiError && err.code === "duplicate_name") {
        toast.error(lt("share.linkNameDuplicate"));
      } else {
        toast.error(tc("error.saveFailed"));
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-5" data-testid="deal-room-access-control-tab">
      <DealRoomAccessRequestsPanel
        roomId={roomId}
        focusLinkId={focusLinkId}
        canManage={canManage}
        onChanged={() => {
          void refetch();
        }}
      />

      {loading || !data ? (
        <p className="py-10 text-center text-sm text-muted-foreground">{tc("loading")}</p>
      ) : (
        <div className="space-y-4" data-testid="room-security-form">
          {!canManage ? (
            <p className="text-sm text-muted-foreground" data-testid="access-policy-oversight-hint">
              {t("accessControl.oversightHint")}
            </p>
          ) : null}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-normal text-muted-foreground">
                {t("accessControl.blocklistTitle")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <ContactEmailTagInput
                values={draft.blockedViewers}
                onChange={(values) => {
                  if (blocklistEnableBlocked && values.length > 0) return;
                  updateDraft({ blockedViewers: values });
                }}
                placeholder={lt("accessRules.blockedViewers.placeholder")}
                hint={
                  blocklistEnableBlocked
                    ? t("accessControl.accessControlsPlanRequired")
                    : lt("accessRules.blockedViewers.roomHint")
                }
                disabled={!canManage || blocklistEnableBlocked}
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-normal text-muted-foreground">
                {t("accessControl.floorsTitle")}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="space-y-1">
                <div className="flex items-center justify-between gap-4">
                  <Label className="text-xs font-normal text-foreground/80">
                    {t("accessControl.floorMustVerify")}
                  </Label>
                  <Switch
                    checked={draft.requireEmailVerification}
                    onCheckedChange={(checked) => {
                      if (checked && verifyEnableBlocked) return;
                      updateDraft({
                        requireEmailVerification: checked,
                        requireEmail: checked ? false : draft.requireEmail,
                      });
                    }}
                    disabled={!canManage || verifyEnableBlocked}
                    aria-label={t("accessControl.floorMustVerify")}
                  />
                </div>
                {verifyEnableBlocked ? (
                  <p className="text-xs text-muted-foreground">
                    {t("accessControl.accessControlsPlanRequired")}
                  </p>
                ) : null}
              </div>
              <div className="space-y-1">
                <div className="flex items-center justify-between gap-4">
                  <Label className="text-xs font-normal text-foreground/80">
                    {t("accessControl.floorMustNda")}
                  </Label>
                  <Switch
                    checked={draft.requireNda}
                    onCheckedChange={(checked) => {
                      if (checked && ndaEnableBlocked) return;
                      updateDraft({ requireNda: checked });
                    }}
                    disabled={!canManage || ndaEnableBlocked}
                    aria-label={t("accessControl.floorMustNda")}
                  />
                </div>
                {ndaEnableBlocked ? (
                  <p className="text-xs text-muted-foreground">{t("accessControl.ndaPlanRequired")}</p>
                ) : null}
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {!loading && data && canManage ? (
        <div className="sticky bottom-[-1.5rem] z-10 -mx-6 border-t bg-background/95 px-6 pt-3 pb-[calc(0.75rem+1.5rem)] backdrop-blur supports-[backdrop-filter]:bg-background/80 md:bottom-[-2rem] md:-mx-8 md:px-8 md:pb-[calc(0.75rem+2rem)]">
          <div className="flex items-center justify-between gap-3">
            <p className="text-xs text-muted-foreground">{t("accessControl.saveHint")}</p>
            <Button
              type="button"
              onClick={() => void handleSave()}
              disabled={saving}
              className="min-w-[140px]"
            >
              {saving ? (
                tc("saving")
              ) : saveSuccess ? (
                <span className="flex items-center gap-1.5">
                  <Check size={16} />
                  {lt("share.savedButtonLabel")}
                </span>
              ) : (
                t("accessControl.saveButton")
              )}
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
