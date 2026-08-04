import { useCallback, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ConfirmDialog } from "@/components/common/ConfirmDialog";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import type { Document } from "@/types";

export type ReplaceDecision = "replace" | "cancel";

/** Thrown when the user cancels a same-name replace prompt. */
export class UploadCancelledError extends Error {
  constructor(message = "upload_cancelled") {
    super(message);
    this.name = "UploadCancelledError";
  }
}

interface ReplacePrompt {
  fileName: string;
  resolve: (decision: ReplaceDecision) => void;
}

/**
 * Shared upload helper: on 409 document_exists, ask Replace/Cancel once,
 * then retry with replace=true. Serializes concurrent prompts so multi-file
 * uploads never stack dialogs.
 */
export function useDocumentUploadConflict() {
  const { t } = useTranslation("documents");
  const [prompt, setPrompt] = useState<ReplacePrompt | null>(null);
  const promptChainRef = useRef(Promise.resolve());

  const askReplace = useCallback((fileName: string) => {
    return new Promise<ReplaceDecision>((resolve) => {
      promptChainRef.current = promptChainRef.current.then(
        () =>
          new Promise<void>((release) => {
            setPrompt({
              fileName,
              resolve: (decision) => {
                resolve(decision);
                release();
              },
            });
          }),
      );
    });
  }, []);

  const uploadDocument = useCallback(
    async (file: File, category?: string): Promise<Document> => {
      try {
        return await api.uploadDocument(file, category);
      } catch (err) {
        if (!(err instanceof ApiError) || err.code !== "document_exists") {
          throw err;
        }
        const decision = await askReplace(file.name);
        if (decision === "cancel") {
          throw new UploadCancelledError(t("upload.replaceCancelled"));
        }
        return api.uploadDocument(file, category, { replace: true });
      }
    },
    [askReplace, t],
  );

  const conflictDialog = (
    <ConfirmDialog
      open={Boolean(prompt)}
      title={t("upload.replaceTitle")}
      description={t("upload.replaceDescription", {
        name: prompt?.fileName ?? "",
      })}
      confirmLabel={t("upload.replaceConfirm")}
      cancelLabel={t("upload.replaceCancel")}
      onConfirm={() => {
        prompt?.resolve("replace");
        setPrompt(null);
      }}
      onCancel={() => {
        prompt?.resolve("cancel");
        setPrompt(null);
      }}
    />
  );

  return { uploadDocument, conflictDialog, isAwaitingConflict: Boolean(prompt) };
}
