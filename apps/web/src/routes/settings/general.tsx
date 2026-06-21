import { useState } from "react";
import { Building, Globe } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function SettingsGeneralPage() {
  const [name, setName] = useState("Acme Capital");
  const [slug, setSlug] = useState("acme-capital");

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Building size={20} />
            工作区信息
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="workspace-name">工作区名称</Label>
            <Input id="workspace-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="workspace-slug">Slug</Label>
            <div className="flex items-center gap-2">
              <Globe size={16} className="text-muted-foreground" />
              <Input id="workspace-slug" value={slug} onChange={(e) => setSlug(e.target.value)} />
            </div>
          </div>
          <Button>保存更改</Button>
        </CardContent>
      </Card>
    </div>
  );
}
