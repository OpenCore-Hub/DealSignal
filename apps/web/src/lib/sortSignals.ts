import type { Signal } from "@/types";

const typeOrder: Record<Signal["type"], number> = {
  hot_signal: 0,
  risk_alert: 1,
  follow_up: 2,
};

const priorityOrder: Record<Signal["priority"], number> = {
  high: 0,
  medium: 1,
  low: 2,
};

export function sortSignals(signals: Signal[]): Signal[] {
  return [...signals].sort((a, b) => {
    const typeDiff = typeOrder[a.type] - typeOrder[b.type];
    if (typeDiff !== 0) return typeDiff;
    const priorityDiff = priorityOrder[a.priority] - priorityOrder[b.priority];
    if (priorityDiff !== 0) return priorityDiff;
    return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
  });
}
