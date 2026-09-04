"use client";

import { useState } from "react";
import { Box, Skeleton } from "@mui/material";
import { useIssueTimeline } from "@/lib/api";
import type { IssueRow, SlaState } from "@/lib/api";
import { gridTemplate } from "@/lib/grid";
import { fmtAge, fmtDateTime, shortPriority, SLA_STATE_LABEL, SLA_STATE_COLOR } from "@/lib/sla";

const MONO = "var(--font-mono)";

const STATE_BG: Record<SlaState, string> = {
  NO_SLA: "var(--sla-no-sla-tint)",
  OK: "var(--sla-ok-tint)",
  AT_RISK: "var(--sla-at-risk-tint)",
  VIOLATED: "var(--sla-violated-tint)",
  TERMINAL: "var(--sla-terminal-tint)",
};

interface IssueTimelineRowProps {
  issue: IssueRow;
  /** Runtime-resolved title; null = unresolvable (show number only). */
  title?: string | null;
  /** True while the batch title request for this list is in flight. */
  titleLoading?: boolean;
  showSlaState?: boolean;
  projectName?: string; // friendly name; falls back to repo short name
  isCsStatus: (status: string | null | undefined) => boolean;
}

export function IssueTimelineRow({
  issue,
  title = null,
  titleLoading = false,
  showSlaState = false,
  projectName,
  isCsStatus,
}: IssueTimelineRowProps) {
  const [open, setOpen] = useState(false);
  const { data: detail } = useIssueTimeline(issue.id, open);

  const cols = gridTemplate(showSlaState);
  const cs = isCsStatus(issue.currentStatus);
  const state = (issue.sla?.slaState ?? "NO_SLA") as SlaState;
  const stateColor = SLA_STATE_COLOR[state];
  const pct = issue.sla?.pctConsumed;
  const budgetColor = stateColor;
  const project = projectName ?? issue.repo?.split("/")[1] ?? "—";

  return (
    <Box sx={{ borderBottom: "1px solid var(--sla-border-soft)", "&:last-of-type": { borderBottom: "none" } }}>
      <Box sx={{ display: "grid", alignItems: "center", px: "22px", py: 1.5, gridTemplateColumns: cols }}>
        {/* Issue */}
        <Box sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: "10px", pr: 1.75 }}>
          <Box
            component="button"
            type="button"
            onClick={() => setOpen((v) => !v)}
            title="Toggle status-event timeline"
            aria-label={open ? "Collapse" : "Expand"}
            sx={{
              m: "-6px", flexShrink: 0, p: "6px", fontSize: 11, color: "var(--sla-no-sla)", transition: "transform 0.15s, color 0.15s",
              transform: open ? "rotate(90deg)" : "rotate(0deg)", background: "none", border: "none", cursor: "pointer",
              "&:hover": { color: "var(--sla-fg2)" },
            }}
          >
            ▶
          </Box>
          <Box
            component="a"
            href={issue.url ?? "#"}
            target="_blank"
            rel="noopener noreferrer"
            title="Open issue on GitHub"
            sx={{ display: "flex", minWidth: 0, alignItems: "center", gap: "10px", color: "inherit", textDecoration: "none", "&:hover": { color: "var(--sla-primary)" } }}
          >
            <Box component="span" sx={{ flexShrink: 0, fontWeight: 600, lineHeight: 1, color: "var(--sla-primary)", fontFamily: MONO, fontSize: 12.5 }}>
              #{issue.number}
            </Box>
            {titleLoading ? (
              <Skeleton variant="text" sx={{ height: 14, width: "100%", maxWidth: 420 }} />
            ) : title ? (
              <Box component="span" sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 13 }}>
                {title}
              </Box>
            ) : null}
          </Box>
        </Box>

        {/* Project */}
        <Box component="span" sx={{ fontSize: 12.5, color: "var(--sla-fg2)" }}>{project}</Box>

        {/* Pri */}
        <Box component="span" sx={{ fontSize: 11, fontWeight: 600, color: "var(--sla-fg2)", fontFamily: MONO }}>
          {shortPriority(issue.priority)}
        </Box>

        {/* Status */}
        <span>
          <Box
            component="span"
            sx={{
              borderRadius: "6px", px: 1.25, py: "3px", fontSize: 11.5, fontWeight: 500,
              bgcolor: cs ? "var(--sla-cs-tint)" : "var(--sla-surface-track)", color: cs ? "var(--sla-cs)" : "var(--sla-fg2)",
            }}
          >
            {issue.currentStatus ?? "—"}
          </Box>
        </span>

        {/* SLA state (issues view only) */}
        {showSlaState && (
          <span>
            <Box component="span" sx={{ borderRadius: "6px", px: 1.25, py: "3px", fontSize: 11.5, fontWeight: 600, bgcolor: STATE_BG[state], color: stateColor }}>
              {SLA_STATE_LABEL[state]}
            </Box>
          </span>
        )}

        {/* Budget */}
        <Box sx={{ display: "flex", alignItems: "center", gap: "10px", pr: 1.75 }}>
          <Box sx={{ position: "relative", height: 7, flex: 1, overflow: "hidden", borderRadius: "4px", bgcolor: "var(--sla-surface-track)" }}>
            <Box sx={{ height: "100%", borderRadius: "4px", width: `${pct != null ? Math.min(pct, 1) * 100 : 0}%`, bgcolor: budgetColor }} />
          </Box>
          <Box sx={{ width: 34, flexShrink: 0, textAlign: "right", fontWeight: 600, lineHeight: 1, fontFamily: MONO, fontSize: 11.5, color: pct != null ? budgetColor : "var(--sla-fg3)" }}>
            {pct != null ? `${Math.round(pct * 100)}%` : "—"}
          </Box>
        </Box>

        {/* Age */}
        <Box component="span" sx={{ textAlign: "right", fontSize: 12, color: "var(--sla-fg3)", fontFamily: MONO }}>
          {fmtAge(issue.githubCreatedAt)}
        </Box>
      </Box>

      {open && (
        <Box sx={{ borderTop: "1px solid var(--sla-border-soft)", bgcolor: "var(--sla-surface-muted)", px: "22px", py: 2, pl: "45px" }}>
          <Box sx={{ mb: 1.75, display: "flex", alignItems: "center", gap: "14px" }}>
            <Box
              component="a"
              href={issue.url ?? "#"}
              target="_blank"
              rel="noopener noreferrer"
              sx={{ fontWeight: 600, lineHeight: 1, color: "var(--sla-primary)", fontFamily: MONO, fontSize: 12.5, textDecoration: "none" }}
            >
              #{issue.number} ↗
            </Box>
            {/* PRIVACY: no "Assigned to" line — assignee data is not collected. */}
          </Box>
          <Box sx={{ mb: 1.5, fontSize: 11, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--sla-fg3)" }}>
            Status-event timeline
          </Box>
          {!detail ? (
            <Box component="p" sx={{ m: 0, fontSize: 12, color: "var(--sla-fg3)" }}>Loading timeline…</Box>
          ) : detail.events.length === 0 ? (
            <Box component="p" sx={{ m: 0, fontSize: 12, color: "var(--sla-fg3)" }}>No status events recorded.</Box>
          ) : (
            <Box sx={{ display: "flex", flexDirection: "column" }}>
              {detail.events.map((e) => (
                <Box key={e.id} sx={{ display: "flex", alignItems: "center", gap: 1.5, py: "7px" }}>
                  {/* The 3px inner ring must match this dot's actual backdrop
                      (the expanded panel's --sla-surface-muted, not a flat
                      white) to keep the "punched gap" illusion correct
                      against a translucent Acrylic surface. */}
                  <Box
                    component="span"
                    sx={{ height: 9, width: 9, flexShrink: 0, borderRadius: "50%", bgcolor: "var(--sla-no-sla)", boxShadow: "0 0 0 3px var(--sla-surface-muted), 0 0 0 4px var(--sla-border)" }}
                  />
                  <Box component="span" sx={{ fontSize: 12.5, color: "var(--sla-fg3)" }}>{e.previousStatus ?? "created"}</Box>
                  <Box component="span" sx={{ color: "var(--sla-no-sla)" }}>→</Box>
                  <Box component="span" sx={{ fontSize: 12.5, fontWeight: 600, color: "var(--sla-fg)" }}>{e.status ?? "—"}</Box>
                  {/* PRIVACY: no actor — transition + timestamp only. */}
                  <Box component="span" sx={{ ml: "auto", fontSize: 12, color: "var(--sla-no-sla)" }}>{fmtDateTime(e.occurredAt)}</Box>
                </Box>
              ))}
            </Box>
          )}
        </Box>
      )}
    </Box>
  );
}
