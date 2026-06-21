import type { HeatLevel } from "@/types";

export function calculateUniqueVisitors(logs: { visitorEmail: string }[]): number {
  return new Set(logs.map((l) => l.visitorEmail)).size;
}

export function calculateHeatDistribution(contacts: { heatLevel: HeatLevel }[]): Record<HeatLevel, number> {
  return contacts.reduce(
    (acc, c) => {
      acc[c.heatLevel] = (acc[c.heatLevel] ?? 0) + 1;
      return acc;
    },
    { hot: 0, warm: 0, cold: 0 } as Record<HeatLevel, number>
  );
}

export function isOverdue(dueAt: string): boolean {
  return new Date(dueAt) < new Date();
}

export function daysOverdue(dueAt: string): number {
  const diff = new Date().getTime() - new Date(dueAt).getTime();
  return Math.max(0, Math.floor(diff / 86400000));
}

export function confidenceLabel(sampleCount: number): string {
  if (sampleCount >= 50) return "高置信度";
  if (sampleCount >= 10) return "中置信度";
  return "低置信度（样本较少）";
}
