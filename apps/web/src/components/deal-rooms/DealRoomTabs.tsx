import { useMemo } from "react";
import { useSearchParams } from "react-router";
import { Files, Shield, ChartLineUp, ChatCircleText } from "@phosphor-icons/react";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useTranslation } from "react-i18next";
import type { DealRoomTab } from "@/hooks/useDealRoomTab";

interface DealRoomTabsProps {
  value?: DealRoomTab;
  onChange?: (tab: DealRoomTab) => void;
}

export function DealRoomTabs({ value, onChange }: DealRoomTabsProps) {
  const { t } = useTranslation("dealRooms");
  const [searchParams, setSearchParams] = useSearchParams();

  const activeTab: DealRoomTab = useMemo(() => {
    const fromProp = value;
    const fromQuery = searchParams.get("tab") as DealRoomTab | null;
    const valid: DealRoomTab[] = ["documents", "permissions", "analytics", "qa"];
    if (fromProp && valid.includes(fromProp)) return fromProp;
    if (fromQuery && valid.includes(fromQuery)) return fromQuery;
    return "documents";
  }, [value, searchParams]);

  const handleChange = (tab: DealRoomTab) => {
    onChange?.(tab);
    const next = new URLSearchParams(searchParams);
    if (tab === "documents") {
      next.delete("tab");
    } else {
      next.set("tab", tab);
    }
    setSearchParams(next, { replace: true });
  };

  const tabs: { value: DealRoomTab; label: string; icon: typeof Files }[] = [
    { value: "documents", label: t("tabs.documents"), icon: Files },
    { value: "permissions", label: t("tabs.permissions"), icon: Shield },
    { value: "analytics", label: t("tabs.analytics"), icon: ChartLineUp },
    { value: "qa", label: t("tabs.qa"), icon: ChatCircleText },
  ];

  return (
    <Tabs value={activeTab} onValueChange={(v) => handleChange(v as DealRoomTab)}>
      <TabsList className="w-full sm:w-auto">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          return (
            <TabsTrigger key={tab.value} value={tab.value} className="gap-1.5">
              <Icon size={16} />
              <span className="hidden sm:inline">{tab.label}</span>
            </TabsTrigger>
          );
        })}
      </TabsList>
    </Tabs>
  );
}
