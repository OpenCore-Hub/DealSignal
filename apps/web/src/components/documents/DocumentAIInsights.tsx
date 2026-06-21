import { Sparkle, FileText, TrendUp, Warning } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/common/EmptyState";
import { useAIStore } from "@/stores/aiStore";
import type { PageAnalytics } from "@/types";

interface DocumentAIInsightsProps {
  documentId: string;
  analytics: PageAnalytics[];
}

export function DocumentAIInsights({ documentId, analytics }: DocumentAIInsightsProps) {
  const { setOpen, sendMessage } = useAIStore();

  const askAI = (question: string) => {
    setOpen(true);
    sendMessage(question, { documentId });
  };

  if (analytics.length === 0) {
    return (
      <EmptyState
        icon={<Sparkle size={48} />}
        title="暂无分析数据"
        description="文档被访问后，AI 将基于真实阅读行为生成关键洞察。"
      />
    );
  }

  const topPage = analytics.reduce((top, current) =>
    current.avgDurationSeconds > top.avgDurationSeconds ? current : top
  );
  const exitRisk = analytics.filter((a) => a.exitRate > 0.08).sort((a, b) => b.exitRate - a.exitRate)[0];

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <Sparkle size={20} className="text-warning-500" />
            AI 关键洞察
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-start gap-3 rounded-lg bg-muted/50 p-3">
            <TrendUp size={18} className="mt-0.5 text-success-500" />
            <div>
              <p className="text-sm font-medium">投资人最关注第 {topPage?.pageNumber ?? 1} 页</p>
              <p className="text-caption text-muted-foreground">
                平均停留 {topPage?.avgDurationSeconds ?? 0} 秒，说明这一页是决策关键页。
              </p>
            </div>
          </div>
          {exitRisk && (
            <div className="flex items-start gap-3 rounded-lg bg-muted/50 p-3">
              <Warning size={18} className="mt-0.5 text-hot-500" />
              <div>
                <p className="text-sm font-medium">第 {exitRisk.pageNumber} 页退出率较高</p>
                <p className="text-caption text-muted-foreground">
                  退出率 {Math.round(exitRisk.exitRate * 100)}%，建议优化内容或补充说明。
                </p>
              </div>
            </div>
          )}
          <div className="flex items-start gap-3 rounded-lg bg-muted/50 p-3">
            <FileText size={18} className="mt-0.5 text-primary" />
            <div>
              <p className="text-sm font-medium">证据驱动的回答</p>
              <p className="text-caption text-muted-foreground">
                所有 AI 结论均附带页码与原文定位，可在左侧“内容”页查看高亮。
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-h2">追问 AI</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" onClick={() => askAI("这份文档最关键的风险点是什么？")}>
              关键风险点
            </Button>
            <Button variant="outline" size="sm" onClick={() => askAI("投资人最关注哪些页面？")}>
              投资人关注点
            </Button>
            <Button variant="outline" size="sm" onClick={() => askAI("我应该如何优化这份文档？")}>
              优化建议
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
