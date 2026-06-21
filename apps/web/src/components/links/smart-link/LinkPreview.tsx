import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Link as LinkIcon, Envelope, Lock, Shield, Warning, Check, Copy } from "@phosphor-icons/react";
import type { Document, PermissionConfig } from "@/types";

interface LinkPreviewProps {
  selectedDocument?: Document;
  config: PermissionConfig;
  generatedLink: string | null;
  copied: boolean;
  creating: boolean;
  onCopy: () => void;
  onCreate: () => void;
}

export function LinkPreview({
  selectedDocument,
  config,
  generatedLink,
  copied,
  creating,
  onCopy,
  onCreate,
}: LinkPreviewProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2 flex items-center gap-2">
          <LinkIcon size={20} />
          预览
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="rounded-md border border-border bg-muted p-3">
          <p className="text-caption text-muted-foreground">文档</p>
          <p className="mt-1 text-sm font-medium">{selectedDocument?.title ?? "未选择"}</p>
        </div>
        <div className="space-y-2 text-sm">
          <div className="flex items-center gap-2">
            <Envelope size={16} className={config.requireEmail ? "text-success-500" : "text-muted-foreground"} />
            <span className={config.requireEmail ? "" : "text-muted-foreground"}>邮箱验证</span>
          </div>
          <div className="flex items-center gap-2">
            <Lock size={16} className={config.passwordEnabled ? "text-success-500" : "text-muted-foreground"} />
            <span className={config.passwordEnabled ? "" : "text-muted-foreground"}>访问密码</span>
          </div>
          <div className="flex items-center gap-2">
            <Shield size={16} className={config.watermarkEnabled ? "text-success-500" : "text-muted-foreground"} />
            <span className={config.watermarkEnabled ? "" : "text-muted-foreground"}>动态水印</span>
          </div>
          <div className="flex items-center gap-2">
            <Warning size={16} className={!config.allowDownload ? "text-success-500" : "text-muted-foreground"} />
            <span className={!config.allowDownload ? "" : "text-muted-foreground"}>禁止下载</span>
          </div>
        </div>

        {generatedLink ? (
          <div className="rounded-md border border-success-500/20 bg-success-500/10 p-3">
            <p className="text-caption flex items-center gap-1 text-success-500">
              <Check size={12} /> 已生成链接
            </p>
            <div className="mt-1 flex items-center gap-2">
              <code className="flex-1 truncate text-sm">{generatedLink}</code>
              <Button
                size="icon"
                variant="ghost"
                aria-label={copied ? "已复制" : "复制链接"}
                onClick={onCopy}
              >
                {copied ? <Check size={14} className="text-success-500" /> : <Copy size={14} />}
              </Button>
            </div>
          </div>
        ) : null}

        <Button className="w-full" disabled={!selectedDocument || creating} onClick={onCreate}>
          {creating ? "创建中..." : generatedLink ? "再次创建" : "创建链接"}
        </Button>
      </CardContent>
    </Card>
  );
}
