import { useCallback, useState } from "react";
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
  status: "uploading" | "processing" | "done" | "error";
  error?: string;
}

export function Uploader() {
  const { t } = useTranslation("documents");
  const [isDragging, setIsDragging] = useState(false);
  const [files, setFiles] = useState<UploadFile[]>([]);

  const handleFiles = useCallback((selectedFiles: FileList | null) => {
    if (!selectedFiles) return;

    const newFiles: UploadFile[] = Array.from(selectedFiles).map((file) => ({
      id: Math.random().toString(36).slice(2),
      file,
      progress: 0,
      status: "uploading",
    }));

    setFiles((prev) => [...prev, ...newFiles]);

    newFiles.forEach((uploadFile) => {
      const interval = setInterval(() => {
        setFiles((prev) =>
          prev.map((f) => {
            if (f.id !== uploadFile.id) return f;
            if (f.status !== "uploading") {
              clearInterval(interval);
              return f;
            }
            return { ...f, progress: Math.min(f.progress + Math.random() * 10, 90) };
          })
        );
      }, 300);

      api
        .uploadDocument(uploadFile.file)
        .then(() => {
          clearInterval(interval);
          setFiles((prev) =>
            prev.map((f) =>
              f.id === uploadFile.id
                ? { ...f, progress: 100, status: "done" }
                : f
            )
          );
        })
        .catch((err: Error) => {
          clearInterval(interval);
          setFiles((prev) =>
            prev.map((f) =>
              f.id === uploadFile.id
                ? { ...f, status: "error", error: err.message }
                : f
            )
          );
        });
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

  const removeFile = (id: string) => {
    setFiles((prev) => prev.filter((f) => f.id !== id));
  };

  const supportedTypes = t("upload.supportedTypes");

  return (
    <div className="space-y-4">
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
          type="file"
          accept={supportedTypes}
          multiple
          className="hidden"
          id="file-upload"
          onChange={(e) => handleFiles(e.target.files)}
        />
        <label htmlFor="file-upload">
          <Button variant="outline" className="mt-4">
            {t("upload.selectFiles")}
          </Button>
        </label>
      </div>

      {files.length > 0 && (
        <ul className="space-y-3">
          {files.map((uploadFile) => (
            <li
              key={uploadFile.id}
              className="flex items-center gap-3 rounded-md border border-border p-3"
            >
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                <File size={20} />
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{uploadFile.file.name}</p>
                <p className="text-caption text-muted-foreground">
                  {(uploadFile.file.size / 1024 / 1024).toFixed(2)} MB
                </p>
                {uploadFile.status !== "done" && uploadFile.status !== "error" && (
                  <Progress value={uploadFile.progress} className="mt-2 h-1.5" />
                )}
                {uploadFile.status === "error" && uploadFile.error && (
                  <p className="mt-1 text-caption text-error-500">{uploadFile.error}</p>
                )}
              </div>
              <div className="flex items-center gap-2">
                {uploadFile.status === "done" && (
                  <span data-testid="upload-success">
                    <Check size={18} weight="bold" className="text-success-500" />
                  </span>
                )}
                {uploadFile.status === "error" && (
                  <Warning size={18} weight="bold" className="text-error-500" />
                )}
                {uploadFile.status === "processing" && (
                  <span className="text-caption text-muted-foreground">{t("upload.processing")}</span>
                )}
                <button
                  onClick={() => removeFile(uploadFile.id)}
                  className="text-muted-foreground hover:text-foreground"
                  aria-label={t("upload.removeFile")}
                >
                  <X size={18} />
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
