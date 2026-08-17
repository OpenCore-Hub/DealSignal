import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { DocumentExistsDialog } from "@/components/documents/DocumentExistsDialog";
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

/** True for 409 document_exists — duck-typed so HMR / bundled duplicates still match. */
export function isDocumentExistsError(err: unknown): boolean {
  if (!err || typeof err !== "object") return false;
  const code = "code" in err ? String((err as { code: unknown }).code) : "";
  if (code === "document_exists") return true;
  return err instanceof ApiError && err.status === 409 && code === "document_exists";
}

/**
 * Shared upload helper: preflight same-name conflict, ask Replace/Cancel,
 * then upload bytes once. 409 document_exists remains a race fallback.
 * Serializes concurrent prompts so multi-file uploads never stack dialogs.
 */
export function useDocumentUploadConflict(opts?: {
  onAwaitingConflictChange?: (awaiting: boolean) => void;
}) {
  const { t } = useTranslation("documents");
  const [prompt, setPrompt] = useState<ReplacePrompt | null>(null);
  const promptRef = useRef<ReplacePrompt | null>(null);
  const settlingRef = useRef(false);
  const promptChainRef = useRef(Promise.resolve());
  const onAwaitingConflictChange = opts?.onAwaitingConflictChange;

  useEffect(() => {
    onAwaitingConflictChange?.(Boolean(prompt));
  }, [prompt, onAwaitingConflictChange]);

  // If the host unmounts mid-prompt (route change), settle as cancel so upload
  // promises cannot hang forever.
  useEffect(() => {
    return () => {
      const current = promptRef.current;
      if (!current || settlingRef.current) return;
      settlingRef.current = true;
      promptRef.current = null;
      current.resolve("cancel");
    };
  }, []);

  const askReplace = useCallback((fileName: string) => {
    return new Promise<ReplaceDecision>((resolve) => {
      const enqueue = () =>
        new Promise<void>((release) => {
          settlingRef.current = false;
          const next: ReplacePrompt = {
            fileName,
            resolve: (decision) => {
              resolve(decision);
              release();
            },
          };
          promptRef.current = next;
          onAwaitingConflictChange?.(true);
          setPrompt(next);
        });

      // Recover if a prior link rejected so the queue cannot stall permanently.
      promptChainRef.current = promptChainRef.current.then(enqueue, enqueue);
    });
  }, [onAwaitingConflictChange]);

  const settle = useCallback((decision: ReplaceDecision) => {
    if (settlingRef.current) return;
    const current = promptRef.current;
    if (!current) return;
    settlingRef.current = true;
    promptRef.current = null;
    onAwaitingConflictChange?.(false);
    setPrompt(null);
    current.resolve(decision);
  }, [onAwaitingConflictChange]);

  const uploadDocument = useCallback(
    async (
      file: File,
      category?: string,
      opts?: { onUploadProgress?: (event: { loaded: number; total: number }) => void },
    ): Promise<Document> => {
      const confirmReplace = async () => {
        const decision = await askReplace(file.name);
        if (decision === "cancel") {
          throw new UploadCancelledError(t("upload.replaceCancelled"));
        }
        return api.uploadDocument(file, category, {
          replace: true,
          ...(opts?.onUploadProgress
            ? { onUploadProgress: opts.onUploadProgress }
            : {}),
        });
      };

      let knownConflict = false;
      try {
        const check = await api.checkDocumentExists(file.name);
        knownConflict = Boolean(check.exists);
      } catch {
        // Preflight is best-effort; upload still handles 409 document_exists.
      }
      if (knownConflict) {
        return confirmReplace();
      }
      try {
        return await (opts?.onUploadProgress
          ? api.uploadDocument(file, category, {
              onUploadProgress: opts.onUploadProgress,
            })
          : api.uploadDocument(file, category));
      } catch (err) {
        if (!isDocumentExistsError(err)) {
          throw err;
        }
        return confirmReplace();
      }
    },
    [askReplace, t],
  );

  const conflictDialog = (
    <DocumentExistsDialog
      open={Boolean(prompt)}
      fileName={prompt?.fileName ?? ""}
      onOverwrite={() => settle("replace")}
      onDiscard={() => settle("cancel")}
    />
  );

  return { uploadDocument, conflictDialog, isAwaitingConflict: Boolean(prompt) };
}
