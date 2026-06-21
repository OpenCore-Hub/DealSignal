import { Plug, CloudArrowUp, Database } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";

const integrations = [
  { id: "slack", name: "Slack", description: "热度提醒推送到 Slack 频道", icon: CloudArrowUp, connected: false },
  { id: "hubspot", name: "HubSpot", description: "同步联系人、访问事件到 CRM", icon: Database, connected: false },
  { id: "zapier", name: "Zapier", description: "自动化工作流触发", icon: Plug, connected: false },
];

export function SettingsIntegrationsPage() {
  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Plug size={20} />
            集成
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="divide-y divide-border">
            {integrations.map((integration) => (
              <li key={integration.id} className="flex items-center justify-between py-4">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-md bg-muted">
                    <integration.icon size={20} className="text-muted-foreground" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">{integration.name}</p>
                    <p className="text-caption text-muted-foreground">{integration.description}</p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <Switch checked={integration.connected} />
                  <Button variant="outline" size="sm">
                    {integration.connected ? "配置" : "连接"}
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
    </div>
  );
}
