type Translate = (key: string, opts?: Record<string, unknown>) => string;

/** Coarse security_events.event_type label (category). */
export function accessEventTypeLabel(t: Translate, eventType: string): string {
  const key = `access.eventTypes.${eventType}`;
  const labeled = t(key);
  return labeled === key ? eventType : labeled;
}

/** Human-readable reason when Insights stores a machine reason code. */
export function accessEventReasonLabel(t: Translate, reason: string | undefined | null): string | null {
  const code = reason?.trim();
  if (!code) return null;
  const key = `access.reasons.${code}`;
  const labeled = t(key);
  if (labeled !== key) return labeled;
  // Unknown codes stay readable (underscores → spaces) rather than raw snake_case only.
  return code.replace(/_/g, " ");
}

/**
 * Primary row label: prefer gate reason for security_gate_failed so hosts see
 * "Email verification required" instead of the opaque category.
 * Other event types keep the type label as primary.
 */
export function accessEventPrimaryLabel(
  t: Translate,
  eventType: string,
  reason?: string | null,
): string {
  const reasonLabel = accessEventReasonLabel(t, reason);
  if (eventType === "security_gate_failed" && reasonLabel) {
    return reasonLabel;
  }
  return accessEventTypeLabel(t, eventType);
}

/**
 * Secondary category under the primary label when reason was promoted.
 * Empty when primary already is the event type (avoid duplicate lines).
 */
export function accessEventSecondaryLabel(
  t: Translate,
  eventType: string,
  reason?: string | null,
): string | null {
  const reasonLabel = accessEventReasonLabel(t, reason);
  if (eventType === "security_gate_failed" && reasonLabel) {
    return accessEventTypeLabel(t, eventType);
  }
  return null;
}
