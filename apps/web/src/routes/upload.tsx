import { Uploader } from "@/components/upload/Uploader";

export function UploadPage() {
  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="space-y-2">
        <h1 className="text-h1">上传文档</h1>
        <p className="text-body text-muted-foreground">
          拖拽文件到下方，或点击选择文件。支持 PDF、Word、PPT、Excel。
        </p>
      </div>
      <Uploader />
    </div>
  );
}
