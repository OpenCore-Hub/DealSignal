import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { motion } from "motion/react";
import {
  Copy,
  LockKeyOpen,
  Lock,
  Shield,
  CaretLeft,
  Link as LinkIcon,
  Envelope,
  Warning,
  FileText,
  Check,
  ShieldCheck,
  ShieldWarning,
} from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import type { Document, PermissionConfig } from "@/types";

type PermissionLevel = "low" | "medium" | "high";

const levelConfig: Record<
  PermissionLevel,
  { label: string; description: string; icon: typeof LockKeyOpen; color: string; friction: string }
> = {
  low: {
    label: "低摩擦",
    description: "公开或邮箱验证即可访问",
    icon: LockKeyOpen,
    color: "text-success-500 bg-success-500/10 border-success-500/20",
    friction: "接收方无需额外步骤，打开率最高。",
  },
  medium: {
    label: "中强度",
    description: "白名单或密码保护",
    icon: Lock,
    color: "text-warm-500 bg-warm-500/10 border-warm-500/20",
    friction: "需要邮箱/密码/白名单验证，适合敏感材料。",
  },
  high: {
    label: "高强度",
    description: "NDA + 白名单 + 密码组合",
    icon: Shield,
    color: "text-hot-500 bg-hot-500/10 border-hot-500/20",
    friction: "NDA 签署 + 多重验证，适合机密尽调资料。",
  },
};

function calculateFrictionScore(config: PermissionConfig): number {
  let score = 0;
  if (config.requireEmail) score += 1;
  if (config.whitelistEnabled) score += 3;
  if (config.passwordEnabled) score += 3;
  if (config.watermarkEnabled) score += 1;
  if (!config.allowDownload) score += 1;
  if (config.expiryDays !== "custom" && config.expiryDays <= 7) score += 1;
  if (config.maxViews !== "unlimited") score += 2;
  return Math.min(10, score);
}

function calculateSecurityScore(config: PermissionConfig): number {
  let score = 0;
  if (config.requireEmail) score += 1;
  if (config.whitelistEnabled) score += 3;
  if (config.passwordEnabled) score += 3;
  if (config.watermarkEnabled) score += 2;
  if (!config.allowDownload) score += 1;
  if (config.maxViews !== "unlimited") score += 1;
  if (config.expiryDays !== "custom" && config.expiryDays <= 30) score += 1;
  return Math.min(10, score);
}

export function SmartLinkCreator() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [searchParams] = useSearchParams();
  const reducedMotion = useReducedMotion();
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loadingDocs, setLoadingDocs] = useState(true);
  const [selectedDocumentId, setSelectedDocumentId] = useState<string>("");
  const [level, setLevel] = useState<PermissionLevel>("low");
  const [config, setConfig] = useState<PermissionConfig>({
    level: "low",
    requireEmail: false,
    whitelistEnabled: false,
    whitelist: [],
    passwordEnabled: false,
    allowDownload: false,
    watermarkEnabled: false,
    expiryDays: 7,
    maxViews: "unlimited",
  });
  const [generatedLink, setGeneratedLink] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    api
      .getDocuments()
      .then((res) => {
        setDocuments(res.data);
        const queryDocId = searchParams.get("documentId");
        const initialId = queryDocId && res.data.some((d) => d.id === queryDocId) ? queryDocId : res.data[0]?.id;
        if (initialId) setSelectedDocumentId(initialId);
      })
      .finally(() => setLoadingDocs(false));
  }, [searchParams]);

  const selectedDocument = useMemo(
    () => documents.find((d) => d.id === selectedDocumentId),
    [documents, selectedDocumentId]
  );

  const handleLevelChange = (value: number | readonly number[]) => {
    const index = Array.isArray(value) ? value[0] : value;
    const levels: PermissionLevel[] = ["low", "medium", "high"];
    const newLevel = levels[index];
    setLevel(newLevel);
    setConfig((prev) => ({
      ...prev,
      level: newLevel,
      requireEmail: newLevel !== "low" || prev.requireEmail,
      whitelistEnabled: newLevel === "high" || (newLevel === "medium" && prev.whitelistEnabled),
      passwordEnabled: newLevel === "high" || (newLevel === "medium" && prev.passwordEnabled),
      watermarkEnabled: newLevel === "high" || prev.watermarkEnabled,
    }));
  };

  const createLink = async () => {
    if (!selectedDocumentId) return;
    setCreating(true);
    try {
      const link = await api.createLink(selectedDocumentId, config);
      setGeneratedLink(link.shortUrl);
    } finally {
      setCreating(false);
    }
  };

  const levelInfo = levelConfig[level];
  const LevelIcon = levelInfo.icon;
  const frictionScore = calculateFrictionScore(config);
  const securityScore = calculateSecurityScore(config);

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      className="mx-auto max-w-4xl space-y-6"
    >
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="sm" onClick={() => navigate(`/${workspaceSlug}/links`)}>
          <CaretLeft size={16} className="mr-1" />
          返回链接列表
        </Button>
      </div>

      <div className="space-y-1">
        <h1 className="text-h1">智能链接创建器</h1>
        <p className="text-body text-muted-foreground">
          选择文档并配置权限强度，系统会实时评估接收方摩擦与安全等级。
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <FileText size={20} />
                选择文档
              </CardTitle>
            </CardHeader>
            <CardContent>
              {loadingDocs ? (
                <Skeleton className="h-10" />
              ) : documents.length === 0 ? (
                <div className="rounded-md border border-dashed border-border p-6 text-center">
                  <p className="text-sm text-muted-foreground">暂无可用文档，请先上传。</p>
                  <Button
                    className="mt-3"
                    size="sm"
                    onClick={() => navigate(`/${workspaceSlug}/documents/upload`)}
                  >
                    上传文档
                  </Button>
                </div>
              ) : (
                <Select value={selectedDocumentId} onValueChange={(value) => value && setSelectedDocumentId(value)}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择要分享的文档" />
                  </SelectTrigger>
                  <SelectContent>
                    {documents.map((doc) => (
                      <SelectItem key={doc.id} value={doc.id}>
                        {doc.title}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <ShieldCheck size={20} />
                权限与安全
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-3">
                <Label>权限强度</Label>
                <Slider
                  value={["low", "medium", "high"].indexOf(level)}
                  onValueChange={handleLevelChange}
                  max={2}
                  step={1}
                />
                <div className={`flex items-center gap-3 rounded-md border p-3 ${levelInfo.color}`}>
                  <LevelIcon size={20} weight="fill" />
                  <div>
                    <p className="text-sm font-medium">{levelInfo.label}</p>
                    <p className="text-caption">{levelInfo.description}</p>
                  </div>
                </div>
              </div>

              <div className="space-y-3">
                <Label>安全选项</Label>
                <div className="space-y-3">
                  <div className="flex items-center gap-2">
                    <Checkbox
                      id="require-email"
                      checked={config.requireEmail}
                      onCheckedChange={(checked) =>
                        setConfig((prev) => ({ ...prev, requireEmail: checked === true }))
                      }
                    />
                    <Label htmlFor="require-email" className="text-sm font-normal">
                      需要邮箱验证
                    </Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <Checkbox
                      id="whitelist"
                      checked={config.whitelistEnabled}
                      onCheckedChange={(checked) =>
                        setConfig((prev) => ({ ...prev, whitelistEnabled: checked === true }))
                      }
                    />
                    <Label htmlFor="whitelist" className="text-sm font-normal">
                      白名单邮箱/域名
                    </Label>
                  </div>
                  {config.whitelistEnabled && (
                    <Input
                      placeholder="输入邮箱或域名，用逗号分隔"
                      value={config.whitelist.join(", ")}
                      onChange={(e) =>
                        setConfig((prev) => ({
                          ...prev,
                          whitelist: e.target.value.split(",").map((s) => s.trim()),
                        }))
                      }
                      className="ml-6"
                    />
                  )}
                  <div className="flex items-center gap-2">
                    <Checkbox
                      id="password"
                      checked={config.passwordEnabled}
                      onCheckedChange={(checked) =>
                        setConfig((prev) => ({ ...prev, passwordEnabled: checked === true }))
                      }
                    />
                    <Label htmlFor="password" className="text-sm font-normal">
                      访问密码
                    </Label>
                  </div>
                  {config.passwordEnabled && (
                    <Input
                      type="password"
                      placeholder="设置密码"
                      value={config.password ?? ""}
                      onChange={(e) => setConfig((prev) => ({ ...prev, password: e.target.value }))}
                      className="ml-6"
                    />
                  )}
                  <div className="flex items-center gap-2">
                    <Checkbox
                      id="download"
                      checked={config.allowDownload}
                      onCheckedChange={(checked) =>
                        setConfig((prev) => ({ ...prev, allowDownload: checked === true }))
                      }
                    />
                    <Label htmlFor="download" className="text-sm font-normal">
                      允许下载
                    </Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <Checkbox
                      id="watermark"
                      checked={config.watermarkEnabled}
                      onCheckedChange={(checked) =>
                        setConfig((prev) => ({ ...prev, watermarkEnabled: checked === true }))
                      }
                    />
                    <Label htmlFor="watermark" className="text-sm font-normal">
                      动态水印
                    </Label>
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label>有效期</Label>
                  <Select
                    value={String(config.expiryDays)}
                    onValueChange={(value) =>
                      setConfig((prev) => ({
                        ...prev,
                        expiryDays: value === "custom" ? "custom" : Number(value),
                      }))
                    }
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="选择有效期" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="7">7 天</SelectItem>
                      <SelectItem value="30">30 天</SelectItem>
                      <SelectItem value="90">90 天</SelectItem>
                      <SelectItem value="custom">自定义</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>最大访问次数</Label>
                  <Select
                    value={String(config.maxViews)}
                    onValueChange={(value) =>
                      setConfig((prev) => ({
                        ...prev,
                        maxViews: value === "unlimited" ? "unlimited" : Number(value),
                      }))
                    }
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="选择最大访问次数" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="unlimited">无限制</SelectItem>
                      <SelectItem value="10">10 次</SelectItem>
                      <SelectItem value="50">50 次</SelectItem>
                      <SelectItem value="100">100 次</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-h2">安全 vs 摩擦</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="flex items-center gap-1">
                    <ShieldWarning size={14} /> 安全强度
                  </span>
                  <span className="font-medium">{securityScore}/10</span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full rounded-full bg-success-500 transition-all"
                    style={{ width: `${securityScore * 10}%` }}
                  />
                </div>
              </div>
              <div>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="flex items-center gap-1">
                    <Warning size={14} /> 接收方摩擦
                  </span>
                  <span className="font-medium">{frictionScore}/10</span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                  <div
                    className={`h-full rounded-full transition-all ${
                      frictionScore <= 3 ? "bg-success-500" : frictionScore <= 6 ? "bg-warm-500" : "bg-hot-500"
                    }`}
                    style={{ width: `${frictionScore * 10}%` }}
                  />
                </div>
              </div>
              <p className="text-caption text-muted-foreground">{levelInfo.friction}</p>
            </CardContent>
          </Card>

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
                      onClick={() => navigator.clipboard.writeText(generatedLink)}
                    >
                      <Copy size={14} />
                    </Button>
                  </div>
                </div>
              ) : null}

              <Button
                className="w-full"
                disabled={!selectedDocumentId || creating}
                onClick={createLink}
              >
                {creating ? "创建中..." : generatedLink ? "再次创建" : "创建链接"}
              </Button>
            </CardContent>
          </Card>
        </div>
      </div>
    </motion.div>
  );
}
