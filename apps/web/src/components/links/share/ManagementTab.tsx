import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ChatTeardropText, FileText, Check, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { FileRequest, VisitorQuestion } from "@/types";
import { formatRelativeTime } from "@/lib/formatters";

interface ManagementTabProps {
  questions: VisitorQuestion[];
  fileRequests: FileRequest[];
  onAnswer: (questionId: string, answer: string) => Promise<void>;
  onUpdateFileRequest: (requestId: string, status: FileRequest["status"]) => Promise<void>;
}

export function ManagementTab({
  questions,
  fileRequests,
  onAnswer,
  onUpdateFileRequest,
}: ManagementTabProps) {
  const { t } = useTranslation("linkShare");
  const [answerDraft, setAnswerDraft] = useState<Record<string, string>>({});
  const [answerLoading, setAnswerLoading] = useState<Record<string, boolean>>({});

  const handleAnswer = async (question: VisitorQuestion) => {
    const text = (answerDraft[question.id] ?? "").trim();
    if (!text) return;
    setAnswerLoading((prev) => ({ ...prev, [question.id]: true }));
    try {
      await onAnswer(question.id, text);
      setAnswerDraft((prev) => ({ ...prev, [question.id]: "" }));
    } finally {
      setAnswerLoading((prev) => ({ ...prev, [question.id]: false }));
    }
  };

  return (
    <div className="space-y-6 py-2">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-h3">
            <ChatTeardropText size={20} />
            {t("management.questionsTitle")}
          </CardTitle>
          <CardDescription>{t("management.questionsDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {questions.length === 0 ? (
            <p className="py-4 text-center text-sm text-muted-foreground">
              {t("management.noQuestions")}
            </p>
          ) : (
            <div className="space-y-4">
              {questions.map((q) => (
                <div key={q.id} className="rounded-lg border p-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0 flex-1 space-y-1">
                      <p className="text-sm font-medium">{q.visitor_email || t("management.anonymous")}</p>
                      <p className="text-sm text-muted-foreground">{q.question}</p>
                      <p className="text-xs text-muted-foreground">
                        {formatRelativeTime(q.created_at)}
                      </p>
                    </div>
                    <Badge variant={q.status === "answered" ? "default" : "warm"}>
                      {t(`management.questionStatus.${q.status}`)}
                    </Badge>
                  </div>
                  {q.answer && (
                    <div className="mt-3 rounded-md bg-muted p-2 text-sm">
                      <span className="font-medium">{t("management.answerLabel")}</span> {q.answer}
                    </div>
                  )}
                  {q.status !== "answered" && (
                    <div className="mt-3 space-y-2">
                      <Textarea
                        value={answerDraft[q.id] ?? ""}
                        onChange={(e) =>
                          setAnswerDraft((prev) => ({ ...prev, [q.id]: e.target.value }))
                        }
                        placeholder={t("management.answerPlaceholder")}
                        rows={2}
                      />
                      <Button
                        size="sm"
                        onClick={() => handleAnswer(q)}
                        disabled={answerLoading[q.id] || !(answerDraft[q.id] ?? "").trim()}
                      >
                        {answerLoading[q.id] ? t("management.saving") : t("management.sendAnswer")}
                      </Button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-h3">
            <FileText size={20} />
            {t("management.fileRequestsTitle")}
          </CardTitle>
          <CardDescription>{t("management.fileRequestsDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {fileRequests.length === 0 ? (
            <p className="py-4 text-center text-sm text-muted-foreground">
              {t("management.noFileRequests")}
            </p>
          ) : (
            <div className="space-y-4">
              {fileRequests.map((req) => (
                <div key={req.id} className="rounded-lg border p-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0 flex-1 space-y-1">
                      <p className="text-sm font-medium">
                        {req.visitor_email || t("management.anonymous")}
                      </p>
                      <p className="text-sm text-muted-foreground">{req.message}</p>
                      <p className="text-xs text-muted-foreground">
                        {formatRelativeTime(req.created_at)}
                      </p>
                    </div>
                    <Badge
                      variant={
                        req.status === "approved" || req.status === "fulfilled"
                          ? "default"
                          : req.status === "rejected"
                          ? "hot"
                          : "warm"
                      }
                    >
                      {t(`management.fileRequestStatus.${req.status}`)}
                    </Badge>
                  </div>
                  <div className="mt-3 flex items-center gap-2">
                    <Select
                      value={req.status}
                      onValueChange={(value) =>
                        onUpdateFileRequest(req.id, value as FileRequest["status"])
                      }
                    >
                      <SelectTrigger className="w-[160px]" aria-label={t("management.status")}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="pending">
                          {t("management.fileRequestStatus.pending")}
                        </SelectItem>
                        <SelectItem value="approved">
                          {t("management.fileRequestStatus.approved")}
                        </SelectItem>
                        <SelectItem value="rejected">
                          {t("management.fileRequestStatus.rejected")}
                        </SelectItem>
                        <SelectItem value="fulfilled">
                          {t("management.fileRequestStatus.fulfilled")}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    {(req.status === "approved" || req.status === "fulfilled") && (
                      <Check size={16} className="text-success-600" />
                    )}
                    {req.status === "rejected" && <X size={16} className="text-destructive" />}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
