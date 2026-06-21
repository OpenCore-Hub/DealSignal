import { CreditCard, Crown } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { UsageBar } from "@/components/common/UsageBar";

export function SettingsBillingPage() {
  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <CreditCard size={20} />
            订阅与用量
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between rounded-lg bg-muted p-4">
            <div>
              <p className="text-sm font-medium">当前方案</p>
              <p className="text-caption text-muted-foreground">Pro 年付</p>
            </div>
            <Button variant="outline" className="gap-1.5">
              <Crown size={16} />
              升级
            </Button>
          </div>

          <div className="space-y-4">
            <UsageBar label="文档存储" current={14.2} max={50} />
            <UsageBar label="分享链接" current={23} max={100} />
            <UsageBar label="数据室" current={2} max={10} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
