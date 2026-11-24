// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, Button, Card, Chip, Drawer, IconButton, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ArrowRight, ChevronDown, History, RefreshCw, X } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useId, useRef, useState, type JSX } from "react";
import { isFeatureEnabled, SLA_TIMELINE_FEATURE_ID } from "@config/featureFlags";
import { formatAbsoluteForUser, getUserPreferredTimeZone } from "@utils/dateTime";
import { buildSlaImpactTimeline, type SlaTimelineEvent } from "../utils/slaImpactTimeline";
import { clusterTimelineEvents, elapsedLabel, ownershipTimeline, timestampMs, type OwnershipPeriod, type OwnershipSide } from "../utils/slaTimelinePresentation";
import type { WorkItemSla } from "../types/workItemSla";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

const COLORS: Record<OwnershipSide, string> = { wso2: "info.main", customer: "warning.main", closed: "text.secondary" };
const SEGMENT_CONTRAST: Record<OwnershipSide, string> = {
  wso2: "info.contrastText",
  customer: "warning.contrastText",
  closed: "background.paper",
};
const LABELS: Record<OwnershipSide, string> = { wso2: "With Choreo Labs", customer: "With customer", closed: "Closed" };
const LATEST_LIMIT = 3;
const when = (ms: number) => formatAbsoluteForUser(new Date(ms).toISOString()) ?? "Unknown time";
const changeText = (event: SlaTimelineEvent) => event.changes.map((c) => `${c.label}: ${c.from ?? "None"} → ${c.to ?? "Cleared"}`).join(" · ");

interface SlaImpactTimelineProps {
  entries: CaseAuditEntry[] | undefined;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  openedAt?: string;
  currentState?: string;
  closedAt?: string;
  slas?: WorkItemSla[];
  isSlasLoading?: boolean;
  isSlasError?: boolean;
}

function TimelineContent({ entries, openedAt, currentState, closedAt, slas = [], isSlasLoading, isSlasError }: SlaImpactTimelineProps): JSX.Element {
  const selectionId = useId();
  const [now, setNow] = useState(() => Date.now());
  // Keep the clock moving so elapsed times and events recorded after mount
  // appear, and refresh it as soon as the audit or case inputs change.
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(timer);
  }, []);
  useEffect(() => setNow(Date.now()), [entries, currentState, closedAt]);
  const plot = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(640);
  const [selection, setSelection] = useState<OwnershipPeriod | null>(null);
  const [history, setHistory] = useState<{ events: SlaTimelineEvent[]; selectedId?: string } | null>(null);
  const historyRefs = useRef(new Map<string, HTMLButtonElement>());
  useEffect(() => {
    const element = plot.current;
    if (!element || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(([entry]) => setWidth(Math.max(1, entry.contentRect.width)));
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!history?.selectedId) return;
    // The drawer's content mounts after its opening transition begins.
    const timer = setTimeout(() => historyRefs.current.get(history.selectedId!)?.scrollIntoView?.({ block: "nearest" }), 0);
    return () => clearTimeout(timer);
  }, [history]);

  const audit = buildSlaImpactTimeline(entries ?? []);
  const allEvents = audit;
  const timeline = ownershipTimeline(allEvents, openedAt, currentState, now, closedAt);
  const events = timeline.events;
  const breachedSlas = slas.filter((sla) => sla.hasBreached);
  const latest = events.slice(-LATEST_LIMIT).reverse();
  const periods = timeline.periods;
  const selected = selection && periods.find((period) => period.start === selection.start && period.end === selection.end) || periods.at(-1);
  const span = Math.max(1, timeline.end - timeline.start);
  const pct = (ms: number) => (ms - timeline.start) / span * 100;
  const regularGroups = clusterTimelineEvents(events, timeline.start, timeline.end, width, 28);

  const openEvent = (event: SlaTimelineEvent) => setHistory({ events: [...events].reverse(), selectedId: event.id });
  const groupLabel = (group: SlaTimelineEvent[]) => group.map((e) => `${e.title}, ${changeText(e)}, ${e.actor}, ${when(timestampMs(e.at)!)}`).join("; ");
  const openGroup = (group: SlaTimelineEvent[]) => openEvent(group.at(-1)!);
  const renderEvent = (event: SlaTimelineEvent, detailed = false) => (
    <Button
      key={event.id}
      ref={(element) => { if (detailed && element) historyRefs.current.set(event.id, element); }}
      onClick={() => detailed ? setHistory((previous) => previous ? { ...previous, selectedId: previous.selectedId === event.id ? undefined : event.id } : null) : openEvent(event)}
      aria-expanded={detailed ? history?.selectedId === event.id : undefined}
      sx={{ textTransform: "none", borderRadius: 0, color: "text.primary", justifyContent: "flex-start", textAlign: "left", width: "100%", py: 1.25, borderBottom: 1, borderColor: "divider", bgcolor: detailed && history?.selectedId === event.id ? "action.selected" : undefined }}
    >
      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, minWidth: 0 }}>
        <Typography variant="caption" color="text.secondary">{when(timestampMs(event.at)!)}</Typography>
        <Typography variant="body2">{event.title}</Typography>
        <Typography variant="caption" color="text.secondary" sx={{ overflowWrap: "anywhere" }}>{changeText(event)}</Typography>
        {detailed && <Typography variant="caption" color="text.secondary">{event.actor || "Unknown actor"}</Typography>}
      </Box>
    </Button>
  );

  return (
    <>
      <Box sx={{ display: "flex", flexWrap: "wrap", gap: 3 }}>
        <Box><Typography variant="caption" color="text.secondary">Case elapsed time</Typography><Typography>{elapsedLabel(timeline.end - timeline.start)}</Typography></Box>
        {(["wso2", "customer"] as const).map((side) => (
          <Box key={side}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}><Box aria-hidden sx={{ width: 10, height: 10, borderRadius: "50%", bgcolor: COLORS[side] }} /><Typography variant="caption" color="text.secondary">{LABELS[side]}</Typography></Box>
            <Typography>{elapsedLabel(periods.filter((p) => p.side === side).reduce((sum, p) => sum + p.end - p.start, 0))}</Typography>
          </Box>
        ))}
      </Box>
      <Typography variant="caption" color="text.secondary">{getUserPreferredTimeZone() ?? Intl.DateTimeFormat().resolvedOptions().timeZone} · elapsed time, not SLA business hours</Typography>
      <Box ref={plot} sx={{ minWidth: 0, px: 1.5 }}>
        <Box aria-hidden sx={{ position: "relative", height: 20 }}>
          {selected && <Box sx={{ position: "absolute", left: `${pct((selected.start + selected.end) / 2)}%`, transform: "translateX(-50%)", display: "flex", color: "text.primary" }}><ChevronDown size={20} strokeWidth={3} /></Box>}
        </Box>
        <Box role="group" aria-label="Ownership timeline" sx={{ display: "flex", alignItems: "center", height: 28, overflow: "visible" }}>
          {periods.map((period, index) => {
            const isSelected = selected?.start === period.start;
            const endRadius = periods.length === 1 ? "999px"
              : index === 0 ? "999px 0 0 999px"
                : index === periods.length - 1 ? "0 999px 999px 0" : "0";
            return (
              <Box
                component="button"
                type="button"
                key={period.start}
                aria-label={`${LABELS[period.side]}, ${elapsedLabel(period.end - period.start)} elapsed, ${when(period.start)} to ${when(period.end)}`}
                aria-pressed={isSelected}
                aria-describedby={isSelected ? selectionId : undefined}
                onClick={() => setSelection(period)}
                sx={{
                  flex: period.end - period.start,
                  height: isSelected ? 24 : 16,
                  borderRadius: isSelected ? "999px" : endRadius,
                  bgcolor: COLORS[period.side],
                  color: SEGMENT_CONTRAST[period.side],
                  minWidth: 0,
                  border: 0,
                  p: 0,
                  cursor: "pointer",
                  position: "relative",
                  zIndex: isSelected ? 1 : 0,
                  boxShadow: isSelected ? 2 : "none",
                  transition: "height 150ms ease, box-shadow 150ms ease",
                  "@media (prefers-reduced-motion: reduce)": { transition: "none" },
                  "&:focus-visible": { outline: "2px dashed", outlineColor: "text.primary", outlineOffset: 2 },
                }}
              />
            );
          })}
        </Box>
        <Box role="group" aria-label="SLA events, oldest to newest" sx={{ position: "relative", height: 24, mt: -0.5 }}>
          {regularGroups.map((group) => <Box component="button" type="button" key={group[0].id} aria-label={groupLabel(group)} onClick={() => openGroup(group)} sx={{ position: "absolute", left: `${pct((timestampMs(group[0].at)! + timestampMs(group.at(-1)!.at)!) / 2)}%`, transform: "translateX(-50%)", width: 24, height: 24, border: 0, bgcolor: "transparent", color: "text.primary", cursor: "pointer", p: 0 }}><Box component="span" sx={{ display: "inline-flex", alignItems: "center", justifyContent: "center", width: group.length > 1 ? 20 : 10, height: group.length > 1 ? 20 : 10, fontSize: 11, border: 1, borderColor: "text.secondary", borderRadius: "50%", bgcolor: "background.paper" }}>{group.length > 1 ? group.length : null}</Box></Box>)}
        </Box>
        <Box sx={{ display: "flex", justifyContent: "space-between", gap: 2 }}><Typography variant="caption" color="text.secondary">Opened<br />{when(timeline.start)}</Typography><Typography variant="caption" color="text.secondary" sx={{ textAlign: "right" }}>{currentState?.toLowerCase() === "closed" || events.at(-1)?.kind === "closed" ? "Closed" : "Now"}<br />{when(timeline.end)}</Typography></Box>
      </Box>
      {breachedSlas.length > 0 && <Box aria-label="Breached SLAs" sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
        {breachedSlas.map((sla) => <Chip key={sla.id} color="error" variant="outlined" size="small" label={`${sla.definition || "SLA"} · Breached`} />)}
      </Box>}
      {isSlasLoading && <Typography variant="caption" color="text.secondary">Loading SLA statuses…</Typography>}
      {isSlasError && <Typography variant="caption" color="error">SLA statuses could not be loaded. The ownership history is still available.</Typography>}
      {selected && <Box id={selectionId} aria-live="polite" sx={{ p: 1.5, bgcolor: "action.selected", borderLeft: 3, borderColor: "primary.main", borderRadius: 1 }}><Typography variant="caption" color="text.secondary">Selected period</Typography><Typography variant="body2">{LABELS[selected.side]} · {elapsedLabel(selected.end - selected.start)} elapsed</Typography><Typography variant="caption" color="text.secondary">{when(selected.start)} → {when(selected.end)}</Typography></Box>}
      <Box><Typography variant="subtitle2">Latest changes</Typography><Typography variant="caption" color="text.secondary">Latest {latest.length} of {events.length} · newest first</Typography>{latest.map((event) => renderEvent(event))}</Box>
      {events.length > LATEST_LIMIT ? <Button size="small" onClick={() => setHistory({ events: [...events].reverse() })} endIcon={<ArrowRight size={14} />} sx={{ alignSelf: "flex-start", textTransform: "none" }}>View all {events.length} events</Button> : events.length === 0 ? <Typography variant="body2" color="text.secondary">No SLA-affecting changes recorded for this case yet.</Typography> : null}
      <Drawer anchor="right" open={!!history} onClose={() => setHistory(null)} PaperProps={{ "aria-label": "SLA event history", sx: { width: { xs: "100%", sm: 440 }, maxWidth: "100%" } }}>
        <Box sx={{ display: "flex", flexDirection: "column", height: "100%", minHeight: 0 }}>
          <Box sx={{ p: 2, display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: 1, borderColor: "divider" }}><Box><Typography variant="subtitle1">SLA event history</Typography><Typography variant="caption" color="text.secondary">{history?.events.length} events · newest first</Typography></Box><IconButton aria-label="Close SLA history" onClick={() => setHistory(null)}><X size={18} /></IconButton></Box>
          <Box sx={{ px: 2, pb: 2, overflowY: "auto", minHeight: 0 }}>{history?.events.map((event) => renderEvent(event, true))}</Box>
        </Box>
      </Drawer>
    </>
  );
}

export function SlaImpactTimeline(props: SlaImpactTimelineProps): JSX.Element | null {
  if (!isFeatureEnabled(SLA_TIMELINE_FEATURE_ID)) return null;
  return (
    <Card component="section" aria-label="SLA impact timeline" sx={{ p: 2.5, mt: 2, display: "flex", flexDirection: "column", gap: 2, minWidth: 0, maxWidth: "100%" }}>
      <Box sx={{ display: "flex", alignItems: "center", flexWrap: "wrap", gap: 1 }}><History size={16} /><Typography variant="subtitle2">SLA impact timeline</Typography></Box>
      {props.isLoading ? <Skeleton variant="rounded" height={180} /> : props.isError ? <Box><Typography color="error">Could not load the SLA impact timeline for this case.</Typography><Button size="small" onClick={props.onRetry} startIcon={<RefreshCw size={14} />}>Retry</Button></Box> : <TimelineContent {...props} />}
    </Card>
  );
}
