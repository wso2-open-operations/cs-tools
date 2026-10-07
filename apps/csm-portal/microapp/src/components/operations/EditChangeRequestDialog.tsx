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
import { AdapterDateFns, Button, Card, DatePickers, Dialog, Stack, Typography } from "@wso2/oxygen-ui";
import { format } from "date-fns";
import type { ChangeRequestDetail } from "@src/types";

const { LocalizationProvider, DateTimePicker } = DatePickers;
const WIRE_FORMAT = "yyyy-MM-dd HH:mm:ss";

interface EditChangeRequestDialogProps {
  changeRequest: ChangeRequestDetail;
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (fields: { plannedStartOn?: string | null }) => void;
}

// Only the planned start is editable here. The customer's approval and review are the CUSTOMER's own decisions, given in the
// Customer Portal: no staff action records them on the customer's behalf (the backend refuses isCustomerApproved /
// isCustomerReviewed from staff with a 400), so this dialog offers no "Customer approved" / "Customer reviewed" switch.
// Save is disabled until the planned start actually differs.
export function EditChangeRequestDialog({
  changeRequest,
  isSubmitting,
  onClose,
  onSubmit,
}: EditChangeRequestDialogProps) {
  const initialPlannedStart = changeRequest.plannedStartOn ? new Date(changeRequest.plannedStartOn) : null;
  const [plannedStart, setPlannedStart] = useState<Date | null>(initialPlannedStart);

  const plannedStartChanged = (plannedStart?.getTime() ?? null) !== (initialPlannedStart?.getTime() ?? null);
  const hasChanges = plannedStartChanged;

  const handleSave = () => {
    onSubmit({
      ...(plannedStartChanged && {
        plannedStartOn: plannedStart ? format(plannedStart, WIRE_FORMAT) : null,
      }),
    });
  };

  return (
    <Dialog
      open
      onClose={onClose}
      slots={{ paper: (props) => <Card component={Stack} {...props} /> }}
      slotProps={{ paper: { sx: { bgcolor: "background.default", p: 1.5, gap: 2, m: 2 } } }}
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
