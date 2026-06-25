import { useCallback, useRef, useState } from "react";
import { UploadSimple, File, X, Check, Warning } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";

interface UploadFile {
  id: string;
  file: File;
  progress: number;
  status: "pending" | "uploading" | "processing" | "done" | "error";
  error?: string;
}

interface UploaderProps {
  onUploadComplete?: () => void;
}

export function Uploader({ onUploadComplete }: UploaderProps) {
  const { t } = useTranslation("documents");
  const [isDragging, setIsDragging] = useState(false);
  const [files, setFiles] = useState<UploadFile[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);

  // Track which files are actively being uploaded to prevent double-upload
  const [uploadingIds, setUploadingIds] = useState<Set<string>>(new Set());

  const openFilePicker = useCallback(() => {
    inputRef.current?.click();
  }, []);

  // Add files to queue with deduplication (by name + size)
  const handleFiles = useCallback((selectedFiles: FileList | null) => {
    if (!selectedFiles || selectedFiles.length === 0) return;

    const existingNames = new Map(
      files.map((f) => [`${f.file.name}|${f.file.size}`, true] as [string, boolean])
    );

    const deduped: UploadFile[] = [];
    for (const file of Array.from(selectedFiles)) {
      const key = `${file.name}|${file.size}`;
      if (existingNames.has(key)) continue; // skip duplicate
      existingNames.set(key, true);
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
  }, [files]);

  // Remove a file from the queue
  const removeFile = useCallback((id: string) => {
    setFiles((prev) => prev.filter((f) => f.id !== id));
  }, []);

  // Upload a single file to the server
  const uploadFileToServer = useCallback(
    (uploadFile: UploadFile): Promise<void> | undefined => {
      if (uploadingIds.has(uploadFile.id)) return; // already uploading

      setUploadingIds((prev) => new Set(prev).add(uploadFile.id));
      setFiles((prev) =>
        prev.map((f) =>
          f.id === uploadFile.id ? { ...f, status: "uploading", error: undefined } : f
        )
      );

      // Simulate progress while waiting for server response
      const interval = setInterval(() => {
        setFiles((prev) =>
          prev.map((f) => {
            if (f.id !== uploadFile.id) return f;
            if (f.status !== "uploading") {
              clearInterval(interval);
              return f;
            }
            return {
              ...f,
              progress: Math.min(f.progress + Math.random() * 15, 95),
            };
          })
        );
      }, 300);

      return api
        .uploadDocument(uploadFile.file)
        .then(() => {
          clearInterval(interval);
          setUploadingIds((prev) => {
            const next = new Set(prev);
            next.delete(uploadFile.id);
            return next;
          });
          setFiles((prev) =>
            prev.map((f) =>
              f.id === uploadFile.id ? { ...f, progress: 100, status: "done" } : f
            )
          );
          onUploadComplete?.();
          // Notify interested components (e.g. document list) that a new upload finished.
          window.dispatchEvent(new CustomEvent("documents:uploaded"));
        })
        .catch((err: Error) => {
          clearInterval(interval);
          setUploadingIds((prev) => {
            const next = new Set(prev);
            next.delete(uploadFile.id);
            return next;
          });
          setFiles((prev) =>
            prev.map((f) =>
              f.id === uploadFile.id
                ? { ...f, status: "error", error: err.message }
                : f
            )
          );
        });
    },
    [uploadingIds, onUploadComplete]
  );

  // Upload all pending files
  const uploadAll = useCallback(async () => {
    const pending = files.filter((f) => f.status === "pending");
    if (pending.length === 0) return;

    // Upload sequentially (could also parallelize with Promise.all)
    for (const uploadFile of pending) {
      await uploadFileToServer(uploadFile);
    }
  }, [files, uploadFileToServer]);

  // Clear completed/error files
  const clearCompleted = useCallback(() => {
    setFiles((prev) => prev.filter((f) => f.status === "pending" || f.status === "uploading"));
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
      {/* Drop zone / file picker */}
      <div
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
        className={cn(
          "flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-10 text-center transition-colors",
          isDragging
            ? "border-primary bg-primary/5"
            : "border-border bg-muted/30 hover:bg-muted/50"
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
          aria-hidden
          className="absolute opacity-0 overflow-hidden w-[1px] h-[1px] p-0 m-[-1px] border-none"
          onChange={(e) => {
            handleFiles(e.target.files);
            e.target.value = "";
          }}
        />
        <Button variant="outline" className="mt-4" onClick={openFilePicker}>
          {t("upload.selectFiles")}
        </Button>
      </div>

      {/* File list with scroll container */}
      {files.length > 0 && (
        <div className="rounded-lg border border-border overflow-hidden">
          {/* Action bar: upload all + clear */}
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
                  disabled={!hasPending}
                  onClick={() => uploadAll()}
                >
                  {hasPending ? t("upload.uploadNow") : t("upload.uploading")}
                </Button>
              )}
            </div>
          </div>

          {/* Scrollable file list */}
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
                      : "border-border hover:bg-muted/50"
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
                      <Progress value={uploadFile.progress} className="mt-2 h-1.5" />
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
                  {uploadFile.status === "uploading" && (
                    <span className="text-caption text-muted-foreground animate-pulse">
                      {Math.round(uploadFile.progress)}%
                    </span>
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
    </div>
  );
}
