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

import { Box, Button, Card, Skeleton, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, ChevronDown, ChevronUp } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import { useLocation, useParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import { useQueryParamTabs } from "@hooks/useSectionTabs";
import QueryErrorState from "@components/QueryErrorState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useAlertMuteRuleMutedAlerts, useGetAlertMuteRule } from "@features/csm-operations/api/useAlertMuteRules";
import {
  ALERT_MUTE_RULE_FIELDS,
  alertMuteRuleState,
  formatDuration,
  timeZoneLabel,
  useMutedAlertsPaging,
  useNow,
} from "@features/csm-operations/utils/alertMuteRules";
import AlertMuteRuleMutedAlerts from "@features/csm-operations/components/AlertMuteRuleMutedAlerts";
import AlertMuteRuleStateChip from "@features/csm-operations/components/AlertMuteRuleStateChip";
import AlertMuteRuleActions from "@features/csm-operations/components/AlertMuteRuleActions";

const LIST_PATH = "/operations/alert-mute-rules";

const RULE_TAB_IDS = ["overview", "muted-alerts"] as const;
type RuleTabId = (typeof RULE_TAB_IDS)[number];

/** Condition fields as the overview labels them; every condition must match for an alert to be muted. */
const CONDITION_LABELS: Record<string, string> = Object.fromEntries(
  ALERT_MUTE_RULE_FIELDS.map((f) => [f.value, f.label.replace(/_/g, " ")]),
);

function formatDateTime(iso: string | null, timeZone?: string): string {
  return iso
    ? (formatBackendTimestampForDisplay(iso, { dateStyle: "medium", timeStyle: "short" }, timeZone) ?? "—")
    : "—";
}

/** One labelled value in the overview grid, styled like the case page's Overview band. */
function Cell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
        {label}
      </Typography>
      <Typography component="div" variant="body2" sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}>
        {children}
      </Typography>
    </Box>
  );
}

/** A collapsible card of label/value cells in a 4-column grid; collapsed, it shows `summary` on one line. */
function OverviewCard({ summary, cells }: { summary: string; cells: Array<[string, ReactNode]> }): JSX.Element {
  const [collapsed, setCollapsed] = useState(false);
  return (
    <Card variant="outlined" sx={{ backgroundColor: "background.default" }}>
      <Box
        component="button"
        type="button"
        onClick={() => setCollapsed((c) => !c)}
        aria-expanded={!collapsed}
        aria-label={collapsed ? "Show alert mute rule details" : "Hide alert mute rule details"}
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          px: 2,
          py: collapsed ? 0.75 : 1,
          width: "100%",
          background: "none",
          border: "none",
          cursor: "pointer",
          color: "inherit",
          textAlign: "left",
          "&:hover": { bgcolor: "action.hover" },
        }}
      >
        {collapsed ? (
          <Typography variant="body2" color="text.secondary" noWrap sx={{ minWidth: 0, mr: 1 }}>
            {summary}
          </Typography>
        ) : (
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ textTransform: "uppercase", letterSpacing: 0.4, fontWeight: 600 }}
          >
            Overview
          </Typography>
        )}
        {collapsed ? <ChevronDown size={18} /> : <ChevronUp size={18} />}
      </Box>
      {!collapsed && (
        <Box
          sx={{
            px: 2,
            pb: 2,
            pt: 1.5,
            borderTop: 1,
            borderColor: "divider",
            display: "grid",
            gridTemplateColumns: {
              xs: "repeat(2, minmax(0, 1fr))",
              sm: "repeat(3, minmax(0, 1fr))",
              md: "repeat(4, minmax(0, 1fr))",
            },
            gap: 2,
          }}
        >
          {cells.map(([label, value]) => (
            <Cell key={label} label={label}>
              {value}
            </Cell>
          ))}
        </Box>
      )}
    </Card>
  );
}

/** One alert mute rule (`/operations/alert-mute-rules/:id`): its details, schedule, what it mutes, and who changed it. */
export default function AlertMuteRuleDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  const location = useLocation();
  const nav = location.state as { from?: string; parentState?: unknown } | undefined;
  const backTarget = nav?.from ?? LIST_PATH;
  const backState = nav?.parentState ?? undefined;
  const now = useNow();
  const { data: rule, isLoading, isError, error } = useGetAlertMuteRule(id);
  const ruleState = rule ? alertMuteRuleState(rule, now) : undefined;
  // Kept in the URL (`?tab=`) so a link to a rule's muted alerts opens on that tab.
  const { activeTab, setActiveTab } = useQueryParamTabs<RuleTabId>(RULE_TAB_IDS, "overview");
  const paging = useMutedAlertsPaging();
  const muted = useAlertMuteRuleMutedAlerts(
    rule && ruleState !== "scheduled" ? id : undefined,
    { limit: paging.rowsPerPage, offset: paging.page * paging.rowsPerPage },
    ruleState === "active",
  );

  const back = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={() => navigate(backTarget, { state: backState })}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {back}
        <Skeleton variant="rounded" height={48} />
        <Skeleton variant="rounded" height={240} />
      </Box>
    );
  }
  if (isError || !rule) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {back}
        <QueryErrorState message={error?.message || "Alert mute rule not found."} error={error} />
      </Box>
    );
  }

  const state = ruleState ?? alertMuteRuleState(rule, now);
  const length = Date.parse(rule.endOn) - Date.parse(rule.startOn);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {back}
      <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
        <Box>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
            <Typography variant="h5">{rule.name}</Typography>
            <AlertMuteRuleStateChip state={state} />
          </Box>
          <Typography variant="body2" color="text.secondary">
            {rule.number}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap", alignItems: "center" }}>
          <AlertMuteRuleActions
          rule={rule}
          state={state}
          variant="buttons"
          returnTo={`${location.pathname}${location.search}`}
          parentState={location.state ?? null}
          onDeleted={() => navigate(backTarget, { replace: true, state: backState })}
        />
        </Box>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v as RuleTabId)} variant="scrollable" scrollButtons="auto">
          <Tab value="overview" label="Overview" sx={{ minHeight: 44, textTransform: "none" }} />
          <Tab
            value="muted-alerts"
            label={`Muted alerts${muted.data ? ` (${muted.data.total})` : ""}`}
            sx={{ minHeight: 44, textTransform: "none" }}
          />
        </Tabs>
      </Box>

      {activeTab === "overview" && (
      <OverviewCard
        summary={[
          ...rule.conditions.map((c) => c.value),
          `${formatDateTime(rule.startOn, rule.timeZone)} → ${formatDateTime(rule.endedOn ?? rule.endOn, rule.timeZone)}`,
        ].join(" · ")}
        cells={[
          ...rule.conditions.map((c): [string, ReactNode] => [CONDITION_LABELS[c.field] ?? c.field, c.value]),
          ["Start", formatDateTime(rule.startOn, rule.timeZone)],
          ["End", formatDateTime(rule.endOn, rule.timeZone)],
          ["Length", formatDuration(length)],
          ["Time zone", timeZoneLabel(rule.timeZone, Date.parse(rule.startOn))],
          ...(rule.endedOn
            ? ([["Ended early", `${formatDateTime(rule.endedOn)} by ${rule.endedBy ?? "—"}`]] as Array<[string, ReactNode]>)
            : []),
          ["Created by", rule.createdBy],
          ["Created", formatDateTime(rule.createdOn)],
          ["Last changed", formatDateTime(rule.updatedOn)],
          ["Description", rule.description || "—"],
        ]}
      />
      )}
      {activeTab === "muted-alerts" && (
        <AlertMuteRuleMutedAlerts
          scheduled={state === "scheduled"}
          data={muted.data}
          isLoading={muted.isLoading}
          error={muted.error}
          page={paging.page}
          rowsPerPage={paging.rowsPerPage}
          onPageChange={paging.setPage}
          onRowsPerPageChange={paging.setRowsPerPage}
        />
      )}
    </Box>
  );
}
