export interface LinkAskSummary {
  total_turns: number;
  ai_answered: number;
  ai_refused: number;
  host_pending: number;
  host_answered: number;
  user_escalated?: number;
  auto_escalated?: number;
  deflection_rate?: number;
  refuse_rate?: number;
  escalation_rate?: number;
}

export function formatAskRate(rate: number | undefined): string {
  if (rate == null || !Number.isFinite(rate)) return "—";
  return `${Math.round(rate * 100)}%`;
}

export function formatAskDeflectionRate(rate: number | undefined): string {
  return formatAskRate(rate);
}

export function hasAskActivity(summary: LinkAskSummary | undefined): boolean {
  return Boolean(summary && summary.total_turns > 0);
}
