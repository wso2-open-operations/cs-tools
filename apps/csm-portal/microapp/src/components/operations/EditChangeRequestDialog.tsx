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

import { useState } from "react";
import { AdapterDateFns, Button, Card, DatePickers, Dialog, Stack, Switch, Typography } from "@wso2/oxygen-ui";
import { format } from "date-fns";
import type { ChangeRequestDetail } from "@src/types";

const { LocalizationProvider, DateTimePicker } = DatePickers;
const WIRE_FORMAT = "yyyy-MM-dd HH:mm:ss";

interface EditChangeRequestDialogProps {
  changeRequest: ChangeRequestDetail;
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (fields: {
    plannedStartOn?: string | null;
    isCustomerApproved?: boolean;
    isCustomerReviewed?: boolean;
  }) => void;
}

// Mirrors the webapp's EditChangeRequestDialog: only 3 fields are editable, matching exactly what
// the backend's PatchChangeRequestPayload accepts (minProperties: 1) — Planned start, Customer
// approved, Customer reviewed. Save is disabled until something actually differs.
export function EditChangeRequestDialog({
  changeRequest,
  isSubmitting,
  onClose,
  onSubmit,
}: EditChangeRequestDialogProps) {
  const initialPlannedStart = changeRequest.plannedStartOn ? new Date(changeRequest.plannedStartOn) : null;
  const [plannedStart, setPlannedStart] = useState<Date | null>(initialPlannedStart);
  const [customerApproved, setCustomerApproved] = useState(changeRequest.hasCustomerApproved);
  const [customerReviewed, setCustomerReviewed] = useState(changeRequest.hasCustomerReviewed);

  const plannedStartChanged = (plannedStart?.getTime() ?? null) !== (initialPlannedStart?.getTime() ?? null);
  const hasChanges =
    plannedStartChanged ||
    customerApproved !== changeRequest.hasCustomerApproved ||
    customerReviewed !== changeRequest.hasCustomerReviewed;

  const handleSave = () => {
    onSubmit({
      ...(plannedStartChanged && {
        plannedStartOn: plannedStart ? format(plannedStart, WIRE_FORMAT) : null,
      }),
      ...(customerApproved !== changeRequest.hasCustomerApproved && { isCustomerApproved: customerApproved }),
      ...(customerReviewed !== changeRequest.hasCustomerReviewed && { isCustomerReviewed: customerReviewed }),
    });
  };

  return (
    <Dialog
      open
      onClose={onClose}
      slots={{ paper: (props) => <Card component={Stack} {...props} /> }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        Edit change request
      </Typography>

      <LocalizationProvider dateAdapter={AdapterDateFns}>
        <DateTimePicker
          label="Planned start"
          value={plannedStart}
          onChange={setPlannedStart}
          slotProps={{ textField: { size: "small", fullWidth: true }, field: { clearable: true } }}
        />
      </LocalizationProvider>

      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="body2">Customer approved</Typography>
        <Switch checked={customerApproved} onChange={(e) => setCustomerApproved(e.target.checked)} />
      </Stack>

      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="body2">Customer reviewed</Typography>
        <Switch checked={customerReviewed} onChange={(e) => setCustomerReviewed(e.target.checked)} />
      </Stack>

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button variant="contained" disabled={!hasChanges || isSubmitting} onClick={handleSave}>
          Save
        </Button>
      </Stack>
    </Dialog>
  );
}
