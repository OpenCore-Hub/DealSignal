import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { motion } from "motion/react";
import {
  Plus,
  Check,
  Folder,
  FileText,
  ShieldCheck,
  LockKeyOpen,
  Lock,
  Shield,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { BackButton } from "@/components/common/BackButton";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import type { DealRoomTemplate } from "@/types";

const permissionIcons = {
  low: LockKeyOpen,
  medium: Lock,
  high: Shield,
};

const permissionLabels = {
  low: "低摩擦",
  medium: "中强度",
  high: "高强度",
};

export function NewDealRoomPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const reducedMotion = useReducedMotion();
  const [templates, setTemplates] = useState<DealRoomTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [nda, setNda] = useState(true);
  const [creating, setCreating] = useState(false);

  const selectTemplate = (template: DealRoomTemplate, fillFields = false) => {
    setSelectedTemplateId(template.id);
    setNda(template.ndaEnabled);
    if (fillFields || !name) setName(template.name);
    if (fillFields || !description) setDescription(template.description);
  };

  useEffect(() => {
    api
      .getDealRoomTemplates()
      .then((res) => {
        setTemplates(res.data);
        if (res.data[0]) selectTemplate(res.data[0], true);
      })
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const selectedTemplate = useMemo(
    () => templates.find((t) => t.id === selectedTemplateId),
    [templates, selectedTemplateId]
  );

  const handleCreate = async () => {
    if (!selectedTemplate || !name) return;
    setCreating(true);
    try {
      const room = await api.createDealRoom({
        name,
        description,
        templateId: selectedTemplate.id,
        ndaEnabled: nda,
      });
      navigate(`/${workspaceSlug}/deal-rooms/${room.id}`);
    } finally {
      setCreating(false);
    }
  };

  return (
    <motion.div
      initial={reducedMotion ? false : { opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      className="mx-auto max-w-5xl space-y-6"
    >
      <BackButton to={`/${workspaceSlug}/deal-rooms`} label="返回 Deal Rooms" />

      <div className="space-y-1">
        <h1 className="text-h1">数据室模板引擎</h1>
        <p className="text-body text-muted-foreground">
          选择场景模板，系统会自动生成文件夹结构与推荐文件清单。
        </p>
      </div>

      {loading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-40" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {templates.map((template) => {
            const selected = selectedTemplateId === template.id;
            return (
              <Card
                key={template.id}
                className={`cursor-pointer transition-all hover:shadow-sm ${
                  selected ? "ring-2 ring-primary" : ""
                }`}
                onClick={() => selectTemplate(template)}
              >
                <CardContent>
                  <div className="flex items-start justify-between">
                    <p className="text-h3">{template.name}</p>
                    {selected && (
                      <div className="flex h-5 w-5 items-center justify-center rounded-full bg-primary text-primary-foreground">
                        <Check size={12} weight="bold" />
                      </div>
                    )}
                  </div>
                  <p className="text-caption mt-2 text-muted-foreground line-clamp-3">
                    {template.description}
                  </p>
                  <div className="mt-3 flex flex-wrap gap-1">
                    <Badge variant="outline" className="text-[10px]">
                      {template.folderStructure.length} 个文件夹
                    </Badge>
                    {template.ndaEnabled && (
                      <Badge variant="secondary" className="text-[10px]">
                        NDA
                      </Badge>
                    )}
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle className="text-h2">基本信息</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="room-name">名称</Label>
              <Input
                id="room-name"
                placeholder="例如：Seed Round Due Diligence"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="room-description">描述</Label>
              <Input
                id="room-description"
                placeholder="说明数据室用途与目标受众"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <div>
                <p className="text-sm font-medium">启用 NDA</p>
                <p className="text-caption text-muted-foreground">访问前要求签署保密协议</p>
              </div>
              <Switch checked={nda} onCheckedChange={setNda} />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="outline" onClick={() => navigate(`/${workspaceSlug}/deal-rooms`)}>
                取消
              </Button>
              <Button className="gap-1.5" disabled={!name || creating} onClick={handleCreate}>
                <Plus size={16} weight="bold" />
                {creating ? "创建中..." : "创建数据室"}
              </Button>
            </div>
          </CardContent>
        </Card>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <Folder size={20} />
                文件夹结构
              </CardTitle>
            </CardHeader>
            <CardContent>
              {selectedTemplate ? (
                <ul className="space-y-2">
                  {selectedTemplate.folderStructure.map((folder, idx) => (
                    <li key={idx} className="flex items-start gap-2 text-sm">
                      <Folder size={16} className="mt-0.5 text-muted-foreground" />
                      <div>
                        <p className="font-medium">{folder.name}</p>
                        {folder.description && (
                          <p className="text-caption text-muted-foreground">{folder.description}</p>
                        )}
                      </div>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-muted-foreground">选择模板后查看文件夹结构。</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <FileText size={20} />
                推荐文件
              </CardTitle>
            </CardHeader>
            <CardContent>
              {selectedTemplate ? (
                <ul className="space-y-2">
                  {selectedTemplate.recommendedFiles.map((file, idx) => (
                    <li key={idx} className="flex items-center gap-2 text-sm">
                      <FileText size={16} className="text-muted-foreground" />
                      {file}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-muted-foreground">选择模板后查看推荐文件。</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <ShieldCheck size={20} />
                默认权限
              </CardTitle>
            </CardHeader>
            <CardContent>
              {selectedTemplate ? (
                <div className="flex items-center gap-2 text-sm">
                  {(() => {
                    const Icon = permissionIcons[selectedTemplate.defaultPermissionLevel];
                    return <Icon size={18} className="text-muted-foreground" />;
                  })()}
                  {permissionLabels[selectedTemplate.defaultPermissionLevel]}
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">选择模板后查看默认权限。</p>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </motion.div>
  );
}
