import { Shield, Key, FileText } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";

export function SettingsSecurityPage() {
  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Shield size={20} />
            安全
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">强制邮箱验证</p>
              <p className="text-caption text-muted-foreground">访问链接前必须验证邮箱</p>
            </div>
            <Switch defaultChecked />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">水印下载</p>
              <p className="text-caption text-muted-foreground">下载 PDF 时附加访客邮箱水印</p>
            </div>
            <Switch />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">双因素认证</p>
              <p className="text-caption text-muted-foreground">为管理员账号启用 2FA</p>
            </div>
            <Button variant="outline" className="gap-1.5">
              <Key size={16} />
              配置
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <FileText size={20} />
            审计日志
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-body text-muted-foreground">
            查看最近 30 天的工作区关键操作记录。
          </p>
          <Button className="mt-4">查看审计日志</Button>
        </CardContent>
      </Card>
    </div>
  );
}
