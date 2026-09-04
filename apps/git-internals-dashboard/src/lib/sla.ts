import type { SlaState } from "./api";

export const SLA_STATES: SlaState[] = ["NO_SLA", "OK", "AT_RISK", "VIOLATED", "TERMINAL"];

export const SLA_STATE_LABEL: Record<SlaState, string> = {
  NO_SLA: "No SLA",
  OK: "On track",
  AT_RISK: "At risk",
  VIOLATED: "Violated",
  TERMINAL: "Closed",
};

// CSS-variable color refs — usable in both SVG (Recharts) and sx/style props.
export const SLA_STATE_COLOR: Record<SlaState, string> = {
  NO_SLA: "var(--sla-no-sla)",
  OK: "var(--sla-ok)",
  AT_RISK: "var(--sla-at-risk)",
  VIOLATED: "var(--sla-violated)",
  TERMINAL: "var(--sla-terminal)",
};

// sx-prop style bundles for badges/dots (mirrors v2's Tailwind utility bundles).
export const SLA_BADGE_SX: Record<SlaState, { borderColor: string; backgroundColor: string; color: string }> = {
  NO_SLA: { borderColor: "color-mix(in srgb, var(--sla-no-sla) 30%, transparent)", backgroundColor: "color-mix(in srgb, var(--sla-no-sla) 10%, transparent)", color: "var(--sla-no-sla)" },
  OK: { borderColor: "color-mix(in srgb, var(--sla-ok) 30%, transparent)", backgroundColor: "color-mix(in srgb, var(--sla-ok) 10%, transparent)", color: "var(--sla-ok)" },
  AT_RISK: { borderColor: "color-mix(in srgb, var(--sla-at-risk) 30%, transparent)", backgroundColor: "color-mix(in srgb, var(--sla-at-risk) 10%, transparent)", color: "var(--sla-at-risk)" },
  VIOLATED: { borderColor: "color-mix(in srgb, var(--sla-violated) 30%, transparent)", backgroundColor: "color-mix(in srgb, var(--sla-violated) 10%, transparent)", color: "var(--sla-violated)" },
  TERMINAL: { borderColor: "color-mix(in srgb, var(--sla-terminal) 30%, transparent)", backgroundColor: "color-mix(in srgb, var(--sla-terminal) 10%, transparent)", color: "var(--sla-terminal)" },
};

export const SLA_DOT_COLOR: Record<SlaState, string> = {
  NO_SLA: "var(--sla-no-sla)",
  OK: "var(--sla-ok)",
  AT_RISK: "var(--sla-at-risk)",
  VIOLATED: "var(--sla-violated)",
  TERMINAL: "var(--sla-terminal)",
};

// "Critical(P1)" → "P1"; falls back to the raw value.
export function shortPriority(priority: string | null | undefined): string {
  if (!priority) return "—";
  return /\((P[1-4])\)/.exec(priority)?.[1] ?? priority;
}

// Budget hours → compact display (≥48h shown as whole days), e.g. 8 → "8h", 120 → "5d".
export function fmtBudget(hours: number | null | undefined): string {
  if (hours == null) return "—";
  if (hours >= 48) return `${Math.round(hours / 24)}d`;
  return `${hours}h`;
}

// Age from creation: < 48h shown in hours, otherwise whole days (matches the design comp).
export function fmtAge(iso: string | null | undefined): string {
  if (!iso) return "—";
  const h = (Date.now() - new Date(iso).getTime()) / 3_600_000;
  if (h < 48) return `${Math.max(0, Math.round(h))}h`;
  return `${Math.round(h / 24)}d`;
}

export function fmtHours(n: number | null | undefined): string {
  if (n == null) return "—";
  return `${Math.round(n * 10) / 10}h`;
}

export function fmtPct(n: number | null | undefined): string {
  if (n == null) return "—";
  return `${Math.round(n * 100)}%`;
}

export function fmtDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}

export function fmtDateTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
