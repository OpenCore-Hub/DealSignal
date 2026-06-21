import { Palette } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function SettingsBrandPage() {
  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Palette size={20} />
            品牌定制
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>Logo</Label>
            <div className="flex h-24 w-24 items-center justify-center rounded-md border border-dashed border-border bg-muted/50 text-muted-foreground">
              上传
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="brand-color">主色</Label>
            <Input id="brand-color" defaultValue="#0f172a" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="viewer-domain">自定义访问域名</Label>
            <Input id="viewer-domain" placeholder="invest.yourdomain.com" />
          </div>
          <Button>保存品牌设置</Button>
        </CardContent>
      </Card>
    </div>
  );
}
