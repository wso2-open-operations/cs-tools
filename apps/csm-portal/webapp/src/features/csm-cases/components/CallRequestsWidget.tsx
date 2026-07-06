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

import {
  Box,
  Button,
  Card,
  Chip,
  Divider,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { Phone, Plus, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { BeCallRequestView, BeCallRequestStateKey } from "@api/backend/types";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import {
  useGetCsmCaseCallRequests,
  usePostCsmCaseCallRequest,
  usePatchCsmCaseCallRequest,
} from "@features/csm-cases/api/useCsmCaseCallRequests";
import {
  ALL_CALL_REQUEST_STATES,
  CALL_REQUEST_STATE_LABEL,
} from "@features/csm-cases/utils/callRequestState";
import { CreateCallRequestDialog } from "./CreateCallRequestDialog";
import { UpdateCallRequestDialog } from "./UpdateCallRequestDialog";
import { CallRequestRow } from "./CallRequestRow";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface CallRequestsWidgetProps {
  caseId: string;
  /** Case severity (S0-S4) — passed to the create dialog to enforce the lead-time rule. */
  severity?: Severity;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function CallRequestsWidget({
  caseId,
  severity,
}: CallRequestsWidgetProps): JSX.Element {
  const { data, isLoading, isError, refetch } = useGetCsmCaseCallRequests(caseId);
  const postCallRequest = usePostCsmCaseCallRequest();
  const patchCallRequest = usePatchCsmCaseCallRequest();

  const [createOpen, setCreateOpen] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const [updateTarget, setUpdateTarget] = useState<BeCallRequestView | null>(null);
  const [updateError, setUpdateError] = useState<string | null>(null);

  // State filter — empty string means "all".
  const [stateFilter, setStateFilter] = useState<BeCallRequestStateKey | "">("");

  const filteredRequests = stateFilter
    ? (data ?? []).filter(
        (cr) => String(cr.state?.id) === stateFilter,
      )
    : (data ?? []);

  const handleCreate = async (
    reason: string,
    utcTimes: string[],
    durationInMinutes: number,
  ) => {
    setCreateError(null);
    try {
      await postCallRequest.mutateAsync({ caseId, reason, utcTimes, durationInMinutes });
      setCreateOpen(false);
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : "Could not submit the call request.",
      );
    }
  };

  const handleUpdateState = async (
    newState: BeCallRequestStateKey,
    cancellationReason?: string,
  ) => {
    if (!updateTarget) return;
    setUpdateError(null);
    try {
      await patchCallRequest.mutateAsync({
        caseId,
        callRequestId: updateTarget.id,
        patch: {
          state: newState,
          ...(cancellationReason ? { cancellationReason } : {}),
        },
      });
      setUpdateTarget(null);
    } catch (err) {
      setUpdateError(
        err instanceof Error ? err.message : "Could not update the call request.",
      );
    }
  };

  return (
    <>
      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        {/* Header */}
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 1,
            flexWrap: "wrap",
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
            <Phone size={16} />
            <Typography variant="subtitle2">Call requests</Typography>
            {!isLoading && !isError && (
              <Chip
                size="small"
                variant="outlined"
                label={`${data?.length ?? 0} total`}
              />
            )}
          </Box>
          <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
            {/* State filter */}
            <FormControl size="small" sx={{ minWidth: 180 }}>
              <InputLabel id="cr-filter-label">Filter by state</InputLabel>
              <Select
                labelId="cr-filter-label"
                value={stateFilter}
                label="Filter by state"
                onChange={(e) =>
                  setStateFilter(e.target.value as BeCallRequestStateKey | "")
                }
              >
                <MenuItem value="">All states</MenuItem>
                <Divider />
                {ALL_CALL_REQUEST_STATES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {CALL_REQUEST_STATE_LABEL[s]}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <Button
              size="small"
              variant="contained"
              startIcon={<Plus size={14} />}
              onClick={() => setCreateOpen(true)}
              sx={{ textTransform: "none" }}
            >
              Request a call
            </Button>
          </Box>
        </Box>

        {/* Content */}
        {isLoading && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} variant="rectangular" height={64} />
            ))}
          </Box>
        )}

        {isError && (
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: 1,
              py: 3,
            }}
          >
            <Typography variant="body2" color="error">
              Could not load call requests.
            </Typography>
            <Button
              size="small"
              variant="outlined"
              startIcon={<RefreshCw size={14} />}
              onClick={() => void refetch()}
              sx={{ textTransform: "none" }}
            >
              Retry
            </Button>
          </Box>
        )}

        {!isLoading && !isError && filteredRequests.length === 0 && (
          <Box sx={{ py: 3, textAlign: "center" }}>
            <Typography variant="body2" color="text.secondary">
              {stateFilter
                ? `No call requests in state "${CALL_REQUEST_STATE_LABEL[stateFilter]}".`
                : "No call requests yet."}
            </Typography>
          </Box>
        )}

        {!isLoading && !isError && filteredRequests.length > 0 && (
          <Box sx={{ display: "flex", flexDirection: "column" }}>
            {filteredRequests.map((cr) => (
              <CallRequestRow
                key={cr.id}
                cr={cr}
                onUpdateState={setUpdateTarget}
              />
            ))}
          </Box>
        )}
      </Card>

      <CreateCallRequestDialog
        open={createOpen}
        submitting={postCallRequest.isPending}
        error={createError}
        severity={severity}
        onClose={() => {
          setCreateOpen(false);
          setCreateError(null);
        }}
        onSubmit={(reason, utcTimes, durationInMinutes) =>
          void handleCreate(reason, utcTimes, durationInMinutes)
        }
      />

      <UpdateCallRequestDialog
        callRequest={updateTarget}
        submitting={patchCallRequest.isPending}
        error={updateError}
        onClose={() => {
          setUpdateTarget(null);
          setUpdateError(null);
        }}
        onSubmit={(newState, cancellationReason) =>
          void handleUpdateState(newState, cancellationReason)
        }
      />
    </>
  );
}
