import { useState } from "react";
import { Copy, LockKeyOpen, Lock, Shield } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle, CardFooter } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { PermissionConfig } from "@/types";

type PermissionLevel = "low" | "medium" | "high";

const levelConfig: Record<
  PermissionLevel,
  { label: string; description: string; icon: typeof LockKeyOpen; color: string }
> = {
  low: {
    label: "低摩擦",
    description: "公开或邮箱验证即可访问",
    icon: LockKeyOpen,
    color: "text-success-500 bg-success-500/10 border-success-500/20",
  },
  medium: {
    label: "中强度",
    description: "白名单或密码保护",
    icon: Lock,
    color: "text-warm-500 bg-warm-500/10 border-warm-500/20",
  },
  high: {
    label: "高强度",
    description: "NDA + 白名单 + 密码组合",
    icon: Shield,
    color: "text-hot-500 bg-hot-500/10 border-hot-500/20",
  },
};

export function PermissionSlider() {
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

  const createLink = () => {
    setGeneratedLink("https://invest.acme.capital/d/X7y8Z9");
  };

  const levelInfo = levelConfig[level];
  const LevelIcon = levelInfo.icon;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2">创建分享链接</CardTitle>
        <p className="text-body text-muted-foreground">
          文档：Acme Pitch Deck.pdf
        </p>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Permission strength slider */}
        <div className="space-y-3">
          <Label>权限强度</Label>
          <Slider
            value={["low", "medium", "high"].indexOf(level)}
            onValueChange={handleLevelChange}
            max={2}
            step={1}
          />
          <div
            className={`flex items-center gap-3 rounded-md border p-3 ${levelInfo.color}`}
          >
            <LevelIcon size={20} weight="fill" />
            <div>
              <p className="text-sm font-medium">{levelInfo.label}</p>
              <p className="text-caption">{levelInfo.description}</p>
            </div>
          </div>
        </div>

        {/* Security options */}
        <div className="space-y-3">
          <Label>安全选项</Label>
          <div className="space-y-2">
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
              <Input type="password" placeholder="设置密码" className="ml-6" />
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

        {/* Expiry and max views */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label>有效期</Label>
            <Select
              value={String(config.expiryDays)}
              onValueChange={(value) =>
                setConfig((prev) => ({ ...prev, expiryDays: Number(value) }))
              }
            >
              <SelectTrigger>
                <SelectValue placeholder="选择有效期" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="7">7 天</SelectItem>
                <SelectItem value="30">30 天</SelectItem>
                <SelectItem value="90">90 天</SelectItem>
                <SelectItem value="-1">自定义</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>最大访问次数</Label>
            <Select
              value={String(config.maxViews)}
              onValueChange={(value) =>
                setConfig((prev) => ({ ...prev, maxViews: value === "unlimited" ? "unlimited" : Number(value) }))
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

        {generatedLink && (
          <div className="rounded-md border border-border bg-muted p-3">
            <p className="text-caption text-muted-foreground">已生成链接</p>
            <div className="mt-1 flex items-center gap-2">
              <code className="flex-1 truncate text-sm">{generatedLink}</code>
              <Button size="icon-xs" variant="ghost" onClick={() => navigator.clipboard.writeText(generatedLink)}>
                <Copy size={14} />
              </Button>
            </div>
          </div>
        )}
      </CardContent>
      <CardFooter className="flex justify-end gap-2">
        <Button variant="outline">取消</Button>
        <Button onClick={createLink}>创建链接</Button>
      </CardFooter>
    </Card>
  );
}
