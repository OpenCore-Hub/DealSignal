import { useState } from "react";
import { Globe } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "react-i18next";
import { setLanguage, getCurrentLanguage } from "@/i18n/utils";
import type { SupportedLanguage } from "@/i18n/config";

export function SettingsLanguagePage() {
  const { t } = useTranslation("settings");
  const { t: tc } = useTranslation("common");
  const [language, setLanguageState] = useState<SupportedLanguage>(getCurrentLanguage());

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Globe size={20} />
            {t("language.title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="workspace-language">{t("language.label")}</Label>
            <Select
              value={language}
              onValueChange={(value) => {
                const lng = value as SupportedLanguage;
                setLanguageState(lng);
                setLanguage(lng);
              }}
            >
              <SelectTrigger id="workspace-language" className="w-full sm:w-64">
                <SelectValue placeholder={t("language.placeholder")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="en">{tc("languages.en")}</SelectItem>
                <SelectItem value="zh-CN">{tc("languages.zh-CN")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <p className="text-caption text-muted-foreground">{t("language.hint")}</p>
        </CardContent>
      </Card>
    </div>
  );
}
