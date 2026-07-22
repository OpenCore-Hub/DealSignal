import { useMemo, useState } from "react";
import { ChatCircleText } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { EmptyState } from "@/components/common/EmptyState";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { formatRelativeTime } from "@/lib/formatters";
import type { Link, VisitorQuestion } from "@/types";

interface DealRoomQATabProps {
  roomId: string;
}

interface RoomAskHostData {
  questions: VisitorQuestion[];
  links: Link[];
}

export function DealRoomQATab({ roomId }: DealRoomQATabProps) {
  const { t } = useTranslation("dealRooms");
  const [linkFilter, setLinkFilter] = useState<string>("all");
  const [answerDraft, setAnswerDraft] = useState<Record<string, string>>({});
  const [answerLoading, setAnswerLoading] = useState<Record<string, boolean>>({});
  const [localOverrides, setLocalOverrides] = useState<Record<string, VisitorQuestion>>({});

  const { data, loading, error, refetch } = useAsyncData(async () => {
    const [questionsRes, linksRes] = await Promise.all([
      api.listRoomQuestions(roomId),
      api.getDealRoomLinks(roomId),
    ]);
    return {
      questions: questionsRes.data ?? [],
      links: linksRes.data ?? [],
    } satisfies RoomAskHostData;
  }, [roomId]);

  const links = useMemo(() => data?.links ?? [], [data?.links]);
  const questions = useMemo(() => {
    const base = data?.questions ?? [];
    if (Object.keys(localOverrides).length === 0) return base;
    return base.map((q) => localOverrides[q.id] ?? q);
  }, [data?.questions, localOverrides]);

  const linkNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const link of links) {
      map.set(link.id, link.name?.trim() || link.documentTitle || link.id);
    }
    return map;
  }, [links]);

  const visibleQuestions = useMemo(() => {
    if (linkFilter === "all") return questions;
    return questions.filter((q) => q.link_id === linkFilter);
  }, [questions, linkFilter]);

  const handleAnswer = async (question: VisitorQuestion) => {
    const text = (answerDraft[question.id] ?? "").trim();
    if (!text) return;
    setAnswerLoading((prev) => ({ ...prev, [question.id]: true }));
    try {
      const res = await api.answerQuestion(question.link_id, question.id, text);
      const updated = res.data ?? { ...question, answer: text, status: "answered" as const };
      setLocalOverrides((prev) => ({ ...prev, [question.id]: updated }));
      setAnswerDraft((prev) => ({ ...prev, [question.id]: "" }));
      toast.success(t("qa.answerSuccess"));
    } catch {
      toast.error(t("qa.answerFailed"));
    } finally {
      setAnswerLoading((prev) => ({ ...prev, [question.id]: false }));
    }
  };

  const handleRetry = async () => {
    setLocalOverrides({});
    await refetch();
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="pb-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-1">
              <CardTitle className="text-h2 flex items-center gap-2">
                <ChatCircleText size={20} />
                {t("qa.title")}
              </CardTitle>
              <CardDescription>{t("qa.description")}</CardDescription>
            </div>
            {links.length > 0 && (
              <Select
                value={linkFilter}
                onValueChange={(value) => {
                  if (value) setLinkFilter(value);
                }}
              >
                <SelectTrigger className="w-[220px]" aria-label={t("qa.filterByLink")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("qa.filterAll")}</SelectItem>
                  {links.map((link) => (
                    <SelectItem key={link.id} value={link.id}>
                      {linkNameById.get(link.id) ?? link.id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p className="py-6 text-center text-sm text-muted-foreground">{t("qa.loading")}</p>
          ) : error ? (
            <div className="space-y-3 py-4 text-center">
              <p className="text-sm text-muted-foreground">{t("qa.loadFailed")}</p>
              <Button variant="outline" size="sm" onClick={() => void handleRetry()}>
                {t("qa.retry")}
              </Button>
            </div>
          ) : visibleQuestions.length === 0 ? (
            <EmptyState
              icon={<ChatCircleText size={40} />}
              title={t("qa.emptyTitle")}
              description={t("qa.emptyDescription")}
            />
          ) : (
            <ul className="space-y-4">
              {visibleQuestions.map((q) => (
                <li key={q.id} className="rounded-lg border border-border bg-card p-4">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0 flex-1 space-y-1">
                      <p className="text-sm font-medium">
                        {q.visitor_email || t("qa.anonymous")}
                      </p>
                      <p className="text-sm text-muted-foreground">{q.question}</p>
                      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                        <span>{formatRelativeTime(q.created_at)}</span>
                        <span aria-hidden>·</span>
                        <span>
                          {t("qa.linkLabel")}: {linkNameById.get(q.link_id) ?? q.link_id}
                        </span>
                      </div>
                    </div>
                    <Badge variant={q.status === "answered" ? "default" : "warm"}>
                      {t(`qa.questionStatus.${q.status}`)}
                    </Badge>
                  </div>
                  {q.answer && (
                    <div className="mt-3 rounded-md bg-muted p-2 text-sm">
                      <span className="font-medium">{t("qa.answerLabel")}</span> {q.answer}
                    </div>
                  )}
                  {q.status !== "answered" && (
                    <div className="mt-3 space-y-2">
                      <Textarea
                        value={answerDraft[q.id] ?? ""}
                        onChange={(e) =>
                          setAnswerDraft((prev) => ({ ...prev, [q.id]: e.target.value }))
                        }
                        placeholder={t("qa.answerPlaceholder")}
                        rows={2}
                      />
                      <Button
                        size="sm"
                        onClick={() => void handleAnswer(q)}
                        disabled={answerLoading[q.id] || !(answerDraft[q.id] ?? "").trim()}
                      >
                        {answerLoading[q.id] ? t("qa.saving") : t("qa.sendAnswer")}
                      </Button>
                    </div>
                  )}
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
