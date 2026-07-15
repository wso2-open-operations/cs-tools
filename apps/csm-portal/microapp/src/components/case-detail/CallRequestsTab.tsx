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

import { Suspense, useState, type ReactNode } from "react";
import {
  AdapterDateFns,
  Button,
  Card,
  Chip,
  DatePickers,
  Dialog,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useQueryClient, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { callRequests } from "@src/services/callRequests";
import type { CallRequest } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { ErrorState } from "@components/support/ErrorState";
import { formatDate } from "@utils/dateTime";

const { LocalizationProvider, DateTimePicker } = DatePickers;

export function CallRequestsTab({ caseId }: { caseId: string }) {
  return (
    <CallRequestsTabErrorBoundary>
      <Suspense fallback={<CallRequestsTabSkeleton />}>
        <CallRequestsTabContent caseId={caseId} />
      </Suspense>
    </CallRequestsTabErrorBoundary>
  );
}

function CallRequestsTabContent({ caseId }: { caseId: string }) {
  const queryClient = useQueryClient();
  const { data: requests } = useSuspenseQuery(callRequests.forCase(caseId));
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <Stack gap={2}>
      <Button variant="contained" size="small" onClick={() => setCreateOpen(true)} sx={{ alignSelf: "start" }}>
        Request a call
      </Button>

      {requests.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No call requests on this case.
        </Typography>
      ) : (
        <Stack gap={1}>
          {requests.map((cr) => (
            <CallRequestRow key={cr.id} callRequest={cr} />
          ))}
        </Stack>
      )}

      {createOpen && (
        <CreateCallRequestDialog
          caseId={caseId}
          onClose={() => setCreateOpen(false)}
          onCreated={() => {
            setCreateOpen(false);
            void queryClient.invalidateQueries({ queryKey: ["case", caseId, "call-requests"] });
          }}
        />
      )}
    </Stack>
  );
}

function CallRequestRow({ callRequest }: { callRequest: CallRequest }) {
  return (
    <Stack gap={0.5} sx={{ p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <Typography variant="body2" fontWeight={500} noWrap sx={{ minWidth: 0 }}>
          {callRequest.number}
        </Typography>
        <Chip size="small" label={callRequest.stateLabel} />
      </Stack>
      <Typography variant="body2" color="text.primary">
        {callRequest.reason}
      </Typography>
      <Typography variant="caption" color="text.secondary">
        {callRequest.durationMin} min
        {callRequest.scheduleTime ? ` · Scheduled ${formatDate(callRequest.scheduleTime)}` : ""}
      </Typography>
      {callRequest.meetingLink && (
        <Typography variant="caption" color="primary.main" noWrap>
          {callRequest.meetingLink}
        </Typography>
      )}
    </Stack>
  );
}

function CreateCallRequestDialog({
  caseId,
  onClose,
  onCreated,
}: {
  caseId: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [reason, setReason] = useState("");
  const [preferredTime, setPreferredTime] = useState<Date | null>(null);
  const [durationMinutes, setDurationMinutes] = useState(30);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canSubmit = reason.trim().length > 0 && preferredTime !== null && durationMinutes > 0 && !isSubmitting;

  const handleSubmit = () => {
    if (!canSubmit || !preferredTime) return;
    setIsSubmitting(true);
    setError(null);
    callRequests
      .create(caseId, {
        reason: reason.trim(),
        utcTimes: [preferredTime.toISOString()],
        durationInMinutes: durationMinutes,
      })
      .then(onCreated)
      .catch(() => setError("Could not request the call. Please try again."))
      .finally(() => setIsSubmitting(false));
  };

  return (
    <Dialog
      open
      onClose={onClose}
      slots={{ paper: (props) => <Card component={Stack} {...props} /> }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        Request a call
      </Typography>

      <TextField
        label="Reason"
        multiline
        minRows={2}
        fullWidth
        size="small"
        value={reason}
        onChange={(e) => setReason(e.target.value)}
      />

      <LocalizationProvider dateAdapter={AdapterDateFns}>
        <DateTimePicker
          label="Preferred time"
          value={preferredTime}
          onChange={setPreferredTime}
          slotProps={{ textField: { size: "small", fullWidth: true } }}
        />
      </LocalizationProvider>

      <TextField
        label="Duration (minutes)"
        type="number"
        size="small"
        fullWidth
        value={durationMinutes}
        onChange={(e) => setDurationMinutes(Math.max(1, Number(e.target.value) || 0))}
      />

      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button variant="contained" disabled={!canSubmit} onClick={handleSubmit}>
          Send
        </Button>
      </Stack>
    </Dialog>
  );
}

function CallRequestsTabSkeleton() {
  return (
    <Stack gap={1}>
      <Skeleton variant="rounded" height={36} width={140} />
      <Skeleton variant="rounded" height={90} />
    </Stack>
  );
}

function CallRequestsTabErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
