import { useCallback, useEffect, useRef, useState } from "react";
import { UploadSimple, File, X, Check, Warning } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { UsageBar } from "@/components/common/UsageBar";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  UploadCancelledError,
  useDocumentUploadConflict,
} from "@/hooks/useDocumentUploadConflict";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { formatFileSize } from "@/lib/formatters";
import { usageAtCap } from "@/lib/planQuota";
import { filterUploadSelection, notifyUploadSelectionFiltered } from "@/lib/uploadFileFilters";
import {
  dispatchDocumentsUploaded,
  isDocumentReadyForLibraryShare,
} from "@/lib/documentsUploadedEvent";
import { ingestionBarPercent, transferBarPercent } from "@/lib/uploadProgress";
import { waitForDocumentIngestion } from "@/lib/waitForDocumentIngestion";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { Document } from "@/types";

interface UploadFile {
  id: string;
  file: File;
  progress: number;
  status: "pending" | "uploading" | "processing" | "done" | "error";
  error?: string;
}

interface UploaderProps {
  /** Fires once after the current upload batch finishes (successful files only). */
  onUploadComplete?: (documents: Document[]) => void;
  category?: string;
  /** Notify host surfaces (e.g. UploadDialog) while the replace prompt is open. */
  onAwaitingConflictChange?: (awaiting: boolean) => void;
}

export function Uploader({
  onUploadComplete,
  category,
  onAwaitingConflictChange,
}: UploaderProps) {
  const { t } = useTranslation("documents");
  const { uploadDocument, conflictDialog, isAwaitingConflict } =
    useDocumentUploadConflict({ onAwaitingConflictChange });
  const { data: billing } = useAsyncData(() => api.getBillingInfo().catch(() => null), []);
  const storageAtCap = billing
    ? usageAtCap(billing.storageUsed, billing.storageLimit)
    : false;
  const documentsAtCap = billing
    ? usageAtCap(billing.documentsUsed ?? 0, billing.documentsLimit ?? 0)
    : false;
  const [isDragging, setIsDragging] = useState(false);
  const [files, setFiles] = useState<UploadFile[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const existingKeysRef = useRef<Set<string>>(new Set());
  const completedDocsRef = useRef<Document[]>([]);
  const batchNotifiedRef = useRef(false);
  const ingestionAbortRef = useRef<Map<string, AbortController>>(new Map());
  const [uploadingIds, setUploadingIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    const ingestionAborts = ingestionAbortRef.current;
    return () => {
      for (const controller of ingestionAborts.values()) {
        controller.abort();
      }
      ingestionAborts.clear();
    };
  }, []);

  const openFilePicker = useCallback(() => {
    inputRef.current?.click();
  }, []);

  const handleFiles = useCallback((selectedFiles: FileList | null) => {
    if (!selectedFiles || selectedFiles.length === 0) return;

    const selection = filterUploadSelection(Array.from(selectedFiles));
    if (!notifyUploadSelectionFiltered(selection, t("upload.blockedFilesSkipped"), toast)) {
      return;
    }
    const filtered = selection.files;

    const deduped: UploadFile[] = [];
    for (const file of filtered) {
      const key = `${file.name}|${file.size}`;
      if (existingKeysRef.current.has(key)) continue;
      existingKeysRef.current.add(key);
      deduped.push({
        id: Math.random().toString(36).slice(2),
        file,
        progress: 0,
        status: "pending",
      });
    }

    if (deduped.length > 0) {
      setFiles((prev) => [...prev, ...deduped]);
    }
  }, [t]);

  const removeFile = useCallback((id: string) => {
    ingestionAbortRef.current.get(id)?.abort();
    ingestionAbortRef.current.delete(id);
    setFiles((prev) => {
      const removed = prev.find((f) => f.id === id);
      if (removed) {
        existingKeysRef.current.delete(`${removed.file.name}|${removed.file.size}`);
      }
      return prev.filter((f) => f.id !== id);
    });
  }, []);

  const markDone = useCallback(
    (uploadId: string, document?: Document) => {
      setUploadingIds((prev) => {
        const next = new Set(prev);
        next.delete(uploadId);
        return next;
      });
      setFiles((prev) =>
        prev.map((f) => (f.id === uploadId ? { ...f, progress: 100, status: "done" } : f)),
      );
      if (document) {
        completedDocsRef.current = [...completedDocsRef.current, document];
      }
    },
    [],
  );

  const markError = useCallback((uploadId: string, message: string) => {
    setUploadingIds((prev) => {
      const next = new Set(prev);
      next.delete(uploadId);
      return next;
    });
    setFiles((prev) =>
      prev.map((f) =>
        f.id === uploadId ? { ...f, status: "error", error: message } : f,
      ),
    );
  }, []);

  const applyProgress = useCallback((uploadId: string, next: number | null) => {
    if (next == null) return;
    setFiles((prev) =>
      prev.map((f) =>
        f.id === uploadId ? { ...f, progress: Math.max(f.progress, next) } : f,
      ),
    );
  }, []);

  const uploadFileToServer = useCallback(
    async (uploadFile: UploadFile): Promise<void> => {
      if (uploadingIds.has(uploadFile.id)) return;

      setUploadingIds((prev) => new Set(prev).add(uploadFile.id));
      setFiles((prev) =>
        prev.map((f) =>
          f.id === uploadFile.id ? { ...f, status: "uploading", error: undefined } : f,
        ),
      );

      try {
        const uploaded = await uploadDocument(uploadFile.file, category, {
          onUploadProgress: ({ loaded, total }) => {
            applyProgress(uploadFile.id, transferBarPercent(loaded, total));
          },
        });
        if (isDocumentReadyForLibraryShare(uploaded.status)) {
          markDone(uploadFile.id, uploaded);
          return;
        }
        setUploadingIds((prev) => {
          const next = new Set(prev);
          next.delete(uploadFile.id);
          return next;
        });
        setFiles((prev) =>
          prev.map((f) =>
            f.id === uploadFile.id
              ? {
                  ...f,
                  status: "processing",
                  progress: ingestionBarPercent(uploaded.progress, f.progress, uploaded.status),
                }
              : f,
          ),
        );
        const controller = new AbortController();
        ingestionAbortRef.current.set(uploadFile.id, controller);
        // Do not await ingestion here — the next file must POST while this one parses.
        void waitForDocumentIngestion({
          initial: uploaded,
          fetchStatus: () =>
            api.getDocumentStatus(uploaded.id, { signal: controller.signal }),
          signal: controller.signal,
          onStatus: (doc) => {
            applyProgress(
              uploadFile.id,
              ingestionBarPercent(doc.progress, 0, doc.status),
            );
          },
        }).then((result) => {
          ingestionAbortRef.current.delete(uploadFile.id);
          if (controller.signal.aborted || result.outcome === "aborted") return;
          if (result.outcome === "ready") {
            markDone(uploadFile.id, result.document);
            return;
          }
          markError(
            uploadFile.id,
            result.outcome === "timeout"
              ? t("documents:upload.processingTimeout")
              : t("documents:upload.ingestionFailed"),
          );
        }).catch(() => {
          ingestionAbortRef.current.delete(uploadFile.id);
        });
      } catch (err) {
        if (err instanceof UploadCancelledError) {
          markError(uploadFile.id, t("documents:upload.replaceCancelled"));
          return;
        }
        markError(uploadFile.id, apiErrorMessage(err, { fallback: "uploadFailed" }));
      }
    },
    [uploadingIds, category, uploadDocument, applyProgress, markDone, markError, t],
  );

  const uploadAll = useCallback(async () => {
    const pending = files.filter((f) => f.status === "pending");
    if (pending.length === 0) return;
    batchNotifiedRef.current = false;
    for (const uploadFile of pending) {
      await uploadFileToServer(uploadFile);
    }
  }, [files, uploadFileToServer]);

  useEffect(() => {
    const inFlight = files.some(
      (file) =>
        file.status === "pending" ||
        file.status === "uploading" ||
        file.status === "processing",
    );
    if (inFlight || batchNotifiedRef.current) return;
    const documents = completedDocsRef.current;
    if (documents.length === 0) return;
    batchNotifiedRef.current = true;
    completedDocsRef.current = [];
    dispatchDocumentsUploaded(
      documents.map((document) => ({
        documentId: document.id,
        documentTitle: document.title || document.fileName || document.id,
        status: document.status,
        category: document.category ?? category,
        createdAt: document.createdAt,
      })),
    );
    onUploadComplete?.(documents);
  }, [files, onUploadComplete, category]);

  const clearCompleted = useCallback(() => {
    setFiles((prev) => {
      const toKeep = prev.filter(
        (f) =>
          f.status === "pending" ||
          f.status === "uploading" ||
          f.status === "processing",
      );
      existingKeysRef.current = new Set(
        toKeep.map((f) => `${f.file.name}|${f.file.size}`),
      );
      return toKeep;
    });
  }, []);

  const onDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const onDragLeave = () => {
    setIsDragging(false);
  };

  const onDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    handleFiles(e.dataTransfer.files);
  };

  const hasPending = files.some((f) => f.status === "pending");
  const hasActive = files.some((f) => f.status === "uploading" || f.status === "processing");
  const hasCompleted = files.some((f) => f.status === "done" || f.status === "error");
  const supportedTypes = t("upload.supportedTypes");

  return (
    <div className="flex flex-col gap-4">
      {billing ? (
        <div className="space-y-1" data-testid="upload-storage-usage">
          <UsageBar
            label={t("upload.storageUsage")}
            current={billing.storageUsed}
            max={billing.storageLimit}
            formatCurrent={formatFileSize(billing.storageUsed)}
            formatMax={formatFileSize(billing.storageLimit)}
          />
          {storageAtCap ? (
            <p className="text-caption text-muted-foreground" data-testid="upload-storage-limit-hint">
              {t("upload.storageLimitReached")}
            </p>
          ) : null}
          <UsageBar
            label={t("upload.documentsUsage")}
            current={billing.documentsUsed ?? 0}
            max={billing.documentsLimit ?? 0}
          />
          {documentsAtCap ? (
            <p className="text-caption text-muted-foreground" data-testid="upload-documents-limit-hint">
              {t("upload.documentsLimitReached")}
            </p>
          ) : null}
        </div>
      ) : null}
      <div
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
        className={cn(
          "flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-10 text-center transition-colors",
          isDragging
            ? "border-primary bg-primary/5"
            : "border-border bg-muted/30 hover:bg-muted/50",
        )}
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
          <UploadSimple size={24} weight="bold" />
        </div>
        <h3 className="mt-4 text-h3">{t("upload.dragTitle")}</h3>
        <p className="mt-1 text-body text-muted-foreground">
          {t("upload.dragDescription")}
        </p>
        <input
          ref={inputRef}
          type="file"
          accept={supportedTypes}
          data-testid="file-upload"
          multiple
          tabIndex={-1}
          className="sr-only"
          onChange={(e) => {
            handleFiles(e.target.files);
            e.target.value = "";
          }}
        />
        <Button variant="outline" className="mt-4" onClick={openFilePicker}>
          {t("upload.selectFiles")}
        </Button>
      </div>

      {files.length > 0 && (
        <div className="rounded-lg border border-border overflow-hidden">
          <div className="flex items-center gap-2 border-b border-border bg-muted/30 px-4 py-2">
            <span className="text-caption text-muted-foreground">
              {t("upload.fileCount", { count: files.length })}
            </span>
            <div className="ml-auto flex items-center gap-2">
              {hasCompleted && (
                <Button variant="ghost" size="sm" onClick={clearCompleted}>
                  {t("upload.clearCompleted")}
                </Button>
              )}
              {(hasPending || hasActive) && (
                <Button
                  size="sm"
                  disabled={!hasPending || isAwaitingConflict}
                  onClick={() => void uploadAll()}
                >
                  {hasPending ? t("upload.uploadNow") : t("upload.uploading")}
                </Button>
              )}
            </div>
          </div>

          <ul className="max-h-[240px] overflow-y-auto space-y-2 p-3">
            {files.map((uploadFile) => (
              <li
                key={uploadFile.id}
                className={cn(
                  "flex items-center gap-3 rounded-md border p-3 transition-colors",
                  uploadFile.status === "error"
                    ? "border-error/30 bg-error/[0.02]"
                    : uploadFile.status === "done"
                      ? "border-success/30 bg-success/[0.02]"
                      : "border-border hover:bg-muted/50",
                )}
              >
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                  <File size={20} />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{uploadFile.file.name}</p>
                  <p className="text-caption text-muted-foreground">
                    {(uploadFile.file.size / 1024 / 1024).toFixed(2)} MB
                  </p>
                  {uploadFile.status !== "done" &&
                    uploadFile.status !== "error" &&
                    uploadFile.status !== "pending" && (
                      <Progress
                        value={uploadFile.progress}
                        className="mt-2 h-1.5"
                        aria-label={t("upload.uploading")}
                        data-testid={
                          uploadFile.status === "processing" ? "upload-processing" : undefined
                        }
                      />
                    )}
                  {uploadFile.status === "error" && uploadFile.error && (
                    <p className="mt-1 text-caption text-error-500 truncate">{uploadFile.error}</p>
                  )}
                </div>
                <div className="flex items-center shrink-0 gap-1">
                  {uploadFile.status === "pending" && (
                    <span className="text-caption text-muted-foreground">
                      {t("upload.pending")}
                    </span>
                  )}
                  {uploadFile.status === "done" && (
                    <Check size={18} weight="bold" className="text-success-500" data-testid="upload-success" />
                  )}
                  {uploadFile.status === "error" && (
                    <Warning size={18} weight="bold" className="text-error-500" />
                  )}
                  <button
                    onClick={() => removeFile(uploadFile.id)}
                    className="text-muted-foreground hover:text-foreground p-1"
                    aria-label={t("upload.removeFile")}
                    disabled={uploadFile.status === "uploading"}
                  >
                    <X size={16} />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}

      {conflictDialog}
    </div>
  );
}
