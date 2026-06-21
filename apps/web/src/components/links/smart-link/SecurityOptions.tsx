import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { PermissionConfig } from "@/types";

interface SecurityOptionsProps {
  config: PermissionConfig;
  onChange: (config: PermissionConfig) => void;
}

export function SecurityOptions({ config, onChange }: SecurityOptionsProps) {
  const update = (patch: Partial<PermissionConfig>) => onChange({ ...config, ...patch });

  return (
    <div className="space-y-6">
      <div className="space-y-3">
        <Label>安全选项</Label>
        <div className="space-y-3">
          <div className="flex items-center gap-2">
            <Checkbox
              id="require-email"
              checked={config.requireEmail}
              onCheckedChange={(checked) => update({ requireEmail: checked === true })}
            />
            <Label htmlFor="require-email" className="text-sm font-normal">
              需要邮箱验证
            </Label>
          </div>
          <div className="flex items-center gap-2">
            <Checkbox
              id="whitelist"
              checked={config.whitelistEnabled}
              onCheckedChange={(checked) => update({ whitelistEnabled: checked === true })}
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
                update({ whitelist: e.target.value.split(",").map((s) => s.trim()) })
              }
              className="ml-6"
            />
          )}
          <div className="flex items-center gap-2">
            <Checkbox
              id="password"
              checked={config.passwordEnabled}
              onCheckedChange={(checked) => update({ passwordEnabled: checked === true })}
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
              onChange={(e) => update({ password: e.target.value })}
              className="ml-6"
            />
          )}
          <div className="flex items-center gap-2">
            <Checkbox
              id="download"
              checked={config.allowDownload}
              onCheckedChange={(checked) => update({ allowDownload: checked === true })}
            />
            <Label htmlFor="download" className="text-sm font-normal">
              允许下载
            </Label>
          </div>
          <div className="flex items-center gap-2">
            <Checkbox
              id="watermark"
              checked={config.watermarkEnabled}
              onCheckedChange={(checked) => update({ watermarkEnabled: checked === true })}
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
              update({ expiryDays: value === "custom" ? "custom" : Number(value) })
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
              update({ maxViews: value === "unlimited" ? "unlimited" : Number(value) })
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
    </div>
  );
}
