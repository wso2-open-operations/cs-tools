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

import { Box, Button, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useLocation, useParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import { useAlertMuteRuleMutedAlerts, useGetAlertMuteRule } from "@features/csm-operations/api/useAlertMuteRules";
import { alertMuteRuleState, useMutedAlertsPaging, useNow } from "@features/csm-operations/utils/alertMuteRules";
import AlertMuteRuleMutedAlerts from "@features/csm-operations/components/AlertMuteRuleMutedAlerts";
import AlertMuteRuleStateChip from "@features/csm-operations/components/AlertMuteRuleStateChip";

const LIST_PATH = "/operations/alert-mute-rules";

/**
 * The alerts one rule muted (`/operations/alert-mute-rules/:id/muted-alerts`), opened from the Muted alerts cell on
 * the rule's Overview. Back returns to the rule with the state it had.
 */
export default function AlertMuteRuleMutedAlertsPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  const location = useLocation();
  const nav = location.state as { from?: string; parentState?: unknown } | undefined;
  const backTarget = nav?.from ?? `${LIST_PATH}/${id ?? ""}`;
  const backState = nav?.parentState ?? undefined;
  const now = useNow();
  const { data: rule, isLoading, isError, error } = useGetAlertMuteRule(id);
  const state = rule ? alertMuteRuleState(rule, now) : undefined;
  const paging = useMutedAlertsPaging();
  const muted = useAlertMuteRuleMutedAlerts(
    rule && state !== "scheduled" ? id : undefined,
    { limit: paging.rowsPerPage, offset: paging.page * paging.rowsPerPage },
    state === "active",
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
  if (isError || !rule || !state) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {back}
        <QueryErrorState message={error?.message || "Alert mute rule not found."} error={error} />
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {back}
      <Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
          <Typography variant="h5">{rule.name}</Typography>
          <AlertMuteRuleStateChip state={state} />
        </Box>
        <Typography variant="body2" color="text.secondary">
          {rule.number}
        </Typography>
      </Box>
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
    </Box>
  );
}
