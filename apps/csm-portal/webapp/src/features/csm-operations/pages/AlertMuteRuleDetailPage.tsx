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

import { Box, Button, Paper, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, BellOff } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import { useLocation, useParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useAlertMuteRuleMutedAlerts, useGetAlertMuteRule } from "@features/csm-operations/api/useAlertMuteRules";
import {
  alertMuteRuleState,
  formatDuration,
  mutedAlertsSummaryText,
  timeZoneLabel,
  useMutedAlertsPaging,
  useNow,
} from "@features/csm-operations/utils/alertMuteRules";
import AlertMuteRuleMutedAlerts from "@features/csm-operations/components/AlertMuteRuleMutedAlerts";
import AlertMuteRuleStateChip from "@features/csm-operations/components/AlertMuteRuleStateChip";
import AlertMuteRuleActions from "@features/csm-operations/components/AlertMuteRuleActions";

const LIST_PATH = "/operations/alert-mute-rules";

function formatDateTime(iso: string | null, timeZone?: string): string {
  return iso
    ? (formatBackendTimestampForDisplay(iso, { dateStyle: "medium", timeStyle: "short" }, timeZone) ?? "—")
    : "—";
}

function Section({ title, rows }: { title: string; rows: Array<[string, ReactNode]> }): JSX.Element {
  return (
    <Paper variant="outlined" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Typography variant="subtitle2">{title}</Typography>
      <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "180px 1fr" }, rowGap: 1, columnGap: 2 }}>
        {rows.map(([label, value]) => (
          <Box key={label} sx={{ display: "contents" }}>
            <Typography variant="body2" color="text.secondary">
              {label}
            </Typography>
            <Typography component="div" variant="body2" sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}>
              {value}
            </Typography>
          </Box>
        ))}
      </Box>
    </Paper>
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
  const paging = useMutedAlertsPaging();
  const [mutedOpen, setMutedOpen] = useState(false);
  const ruleState = rule ? alertMuteRuleState(rule, now) : undefined;
  const muted = useAlertMuteRuleMutedAlerts(
    rule ? id : undefined,
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
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
            <Typography variant="h5">{rule.name}</Typography>
            <AlertMuteRuleStateChip state={state} />
          </Box>
          <Typography variant="body2" color="text.secondary">
            {rule.number}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap", alignItems: "center" }}>
          <Button
            variant="outlined"
            size="small"
            startIcon={<BellOff size={16} />}
            onClick={() => setMutedOpen(true)}
            disabled={state === "scheduled"}
            title={state === "scheduled" ? "Nothing is muted before the rule starts" : undefined}
          >
            {`Muted alerts${muted.data ? ` (${muted.data.total})` : ""}`}
          </Button>
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

      <Section
        title="Alert mute rule details"
        rows={[
          ["Name", rule.name],
          ["Description", rule.description || "—"],
          ["Created", `${formatDateTime(rule.createdOn)} by ${rule.createdBy}`],
          ["Last changed", formatDateTime(rule.updatedOn)],
        ]}
      />
      <Section
        title="What it mutes (every condition must match)"
        rows={rule.conditions.map((c): [string, ReactNode] => [c.field, c.value])}
      />
      <Section
        title="Schedule"
        rows={[
          ["Time zone", timeZoneLabel(rule.timeZone, Date.parse(rule.startOn))],
          ["Start", formatDateTime(rule.startOn, rule.timeZone)],
          ["End", formatDateTime(rule.endOn, rule.timeZone)],
          ["Length", formatDuration(length)],
          ...(rule.endedOn ? ([["Ended early", `${formatDateTime(rule.endedOn)} by ${rule.endedBy ?? "—"}`]] as Array<[string, ReactNode]>) : []),
        ]}
      />
      <AlertMuteRuleMutedAlerts
        open={mutedOpen}
        onClose={() => setMutedOpen(false)}
        title={`Muted alerts · ${rule.number}${muted.data ? ` · ${mutedAlertsSummaryText(muted.data)}` : ""}`}
        data={muted.data}
        isLoading={muted.isLoading}
        error={muted.error}
        page={paging.page}
        rowsPerPage={paging.rowsPerPage}
        onPageChange={paging.setPage}
        onRowsPerPageChange={paging.setRowsPerPage}
      />
    </Box>
  );
}
