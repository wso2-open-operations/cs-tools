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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
  Typography,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { usePatchChangeRequest } from "@features/operations/api/usePatchChangeRequest";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { resolveDisplayTimeZone, formatBackendTimestampForDisplay } from "@utils/dateTime";
import {
  callRequestApiPreferredTimeToDatetimeLocal,
  computeMinScheduleDatetimeLocalForTimeZone,
  datetimeLocalWallTimeToUtcMs,
} from "@features/support/utils/support";
import type {
  ChangeRequestDetails,
  ProposeNewImplementationTimeModalProps,
} from "@features/operations/types/changeRequests";

type ProposeNewImplementationTimeModalBodyProps = {
  changeRequest: ChangeRequestDetails;
  onClose: () => void;
};

/**
 * Mounted only while the dialog is open: `proposedStart` initializes from `changeRequest`
 * on mount (no `setState` in `useEffect`).
 */
function ProposeNewImplementationTimeModalBody({
  changeRequest,
  onClose,
}: ProposeNewImplementationTimeModalBodyProps): JSX.Element {
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const patchMutation = usePatchChangeRequest(changeRequest.id);
  const userTimeZone = resolveDisplayTimeZone();
  const [proposedStart, setProposedStart] = useState(() =>
    callRequestApiPreferredTimeToDatetimeLocal(changeRequest.startDate, userTimeZone),
  );

  const isModalBusy = patchMutation.isPending;
  const minDatetime = computeMinScheduleDatetimeLocalForTimeZone(0, userTimeZone);

  const handleClose = () => {
    if (isModalBusy) return;
    onClose();
  };

  const handleSubmit = () => {
    if (!proposedStart) {
      showError("Please select proposed start date and time.");
      return;
    }
    const utcMs = datetimeLocalWallTimeToUtcMs(proposedStart, userTimeZone);
    if (utcMs == null) {
      showError("Invalid date and time. Please select a valid value.");
      return;
    }
    const pad = (n: number) => String(n).padStart(2, "0");
    const d = new Date(utcMs);
    const plannedStartOn = `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`;

    patchMutation.mutate(
      { plannedStartOn },
      {
        onSuccess: () => {
          showSuccess("Implementation schedule updated successfully");
          window.location.reload();
        },
        onError: (err) => {
          showError(err?.message ?? "Failed to submit proposal");
        },
      },
    );
  };

  return (
    <Dialog
      open
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="propose-implementation-dialog-title"
    >
      <DialogTitle
        id="propose-implementation-dialog-title"
        sx={{
          pr: 6,
          position: "relative",
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
        }}
      >
        <Box>
          <Typography variant="h6" component="span" display="block">
            Propose New Implementation Time
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            Suggest an alternative schedule for this change request
          </Typography>
        </Box>
        <IconButton
          aria-label="Close"
          onClick={handleClose}
          size="small"
          sx={{ position: "absolute", right: 8, top: 8 }}
        >
          <X size={20} aria-hidden />
        </IconButton>
      </DialogTitle>
      <DialogContent sx={{ pt: 1 }}>
        <Box
          sx={{
            mb: 2,
            p: 2,
            bgcolor: alpha(colors.grey[500], 0.08),
            border: 1,
            borderColor: "divider",
          }}
        >
          <Typography
            variant="subtitle2"
            color="text.secondary"
            sx={{ mb: 1.5 }}
          >
            Current Schedule
          </Typography>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
              gap: 2,
            }}
          >
            <Box>
              <Typography
                variant="caption"
                color="text.secondary"
                display="block"
              >
                Start Date & Time
              </Typography>
              <Typography variant="body2" color="text.primary">
                {formatBackendTimestampForDisplay(
                  changeRequest.startDate,
                  { weekday: "long", year: "numeric", month: "long", day: "numeric", hour: "numeric", minute: "2-digit", timeZoneName: "short" },
                  userTimeZone,
                ) ?? "Not available"}
              </Typography>
            </Box>
            <Box>
              <Typography
                variant="caption"
                color="text.secondary"
                display="block"
              >
                End Date & Time
              </Typography>
              <Typography variant="body2" color="text.primary">
                {formatBackendTimestampForDisplay(
                  changeRequest.endDate,
                  { weekday: "long", year: "numeric", month: "long", day: "numeric", hour: "numeric", minute: "2-digit", timeZoneName: "short" },
                  userTimeZone,
                ) ?? "Not available"}
              </Typography>
            </Box>
          </Box>
        </Box>

        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1.5 }}>
          Proposed implementation start
        </Typography>
        <Box sx={{ mb: 2 }}>
          <Typography variant="body2" sx={{ mb: 1 }}>
            Proposed start{" "}
            <Box component="span" sx={{ color: "error.main" }}>
              *
            </Box>
          </Typography>
          <TextField
            type="datetime-local"
            size="small"
            fullWidth
            value={proposedStart}
            onChange={(e) => setProposedStart(e.target.value)}
            inputProps={{ min: minDatetime }}
            helperText={`Timezone: ${userTimeZone}`}
          />
        </Box>
      </DialogContent>
      <DialogActions
        sx={{ px: 3, py: 2, borderTop: 1, borderColor: "divider" }}
      >
        <Button
          variant="outlined"
          color="inherit"
          onClick={handleClose}
          disabled={isModalBusy}
        >
          Cancel
        </Button>
        <Button
          variant="contained"
          color="primary"
          onClick={handleSubmit}
          disabled={patchMutation.isPending}
        >
          {patchMutation.isPending ? "Submitting..." : "Submit Proposal"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/**
 * Modal to propose a new implementation window (planned start and end).
 * Current schedule matches Scheduled Maintenance Window planned start/end.
 *
 * @param props - Dialog state and CR.
 * @returns {JSX.Element} The modal.
 */
export default function ProposeNewImplementationTimeModal({
  open,
  onClose,
  changeRequest,
}: ProposeNewImplementationTimeModalProps): JSX.Element | null {
  if (!changeRequest) return null;

  return open ? (
    <ProposeNewImplementationTimeModalBody
      key={changeRequest.id}
      changeRequest={changeRequest}
      onClose={onClose}
    />
  ) : null;
}
