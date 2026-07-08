import { useState } from "react";
import { ChatCircleText, PaperPlaneRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/common/EmptyState";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

interface QAItem {
  id: string;
  question: string;
  answer?: string;
  answeredBy?: string;
  askedAt: string;
}

interface DealRoomQATabProps {
  initialQuestions?: QAItem[];
}

export function DealRoomQATab({ initialQuestions }: DealRoomQATabProps) {
  const { t } = useTranslation("dealRooms");
  const [questions] = useState<QAItem[]>(
    initialQuestions ?? [
      {
        id: "1",
        question: "When is the next board meeting?",
        answer: "Scheduled for next Tuesday at 10 AM PT.",
        answeredBy: "Admin",
        askedAt: "2026-07-05T10:00:00.000Z",
      },
    ]
  );
  const [draft, setDraft] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!draft.trim()) return;
    toast.info(t("qa.comingSoon"));
    setDraft("");
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-h2 flex items-center gap-2">
            <ChatCircleText size={20} />
            {t("qa.title")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex items-center gap-2">
            <Input
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder={t("qa.placeholder")}
              className="flex-1"
            />
            <Button type="submit" disabled={!draft.trim()} className="gap-1.5">
              <PaperPlaneRight size={16} />
              {t("qa.askQuestion")}
            </Button>
          </form>
        </CardContent>
      </Card>

      {questions.length === 0 ? (
        <EmptyState
          icon={<ChatCircleText size={40} />}
          title={t("qa.emptyTitle")}
          description={t("qa.emptyDescription")}
        />
      ) : (
        <ul className="space-y-3">
          {questions.map((q) => (
            <li key={q.id} className="rounded-lg border border-border bg-card p-4">
              <p className="text-sm font-medium">{q.question}</p>
              {q.answer && (
                <p className="mt-2 text-body text-muted-foreground">{q.answer}</p>
              )}
              <div className="mt-2 flex items-center gap-2 text-caption text-muted-foreground">
                {q.answeredBy && <span>{t("qa.answeredBy", { name: q.answeredBy })}</span>}
                <span>·</span>
                <span>{new Date(q.askedAt).toLocaleDateString()}</span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
