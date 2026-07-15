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
import { Button, Card, Dialog, FormControl, InputLabel, MenuItem, Select, Stack, Typography } from "@wso2/oxygen-ui";
import type { CaseSeverity } from "@src/types";
import { ALL_SEVERITIES, SEVERITY_LABELS } from "@components/support/config";

interface ChangeSeverityDialogProps {
  currentSeverity: CaseSeverity;
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (next: CaseSeverity) => void;
}

export function ChangeSeverityDialog({ currentSeverity, isSubmitting, onClose, onSubmit }: ChangeSeverityDialogProps) {
  const [severity, setSeverity] = useState<CaseSeverity>(currentSeverity);

  return (
    <Dialog
      open
      onClose={onClose}
      slots={{ paper: (props) => <Card component={Stack} {...props} /> }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        Change severity
      </Typography>

      <FormControl size="small" fullWidth>
        <InputLabel id="change-severity-label">Severity</InputLabel>
        <Select
          labelId="change-severity-label"
          label="Severity"
          value={severity}
          onChange={(e) => setSeverity(e.target.value as CaseSeverity)}
        >
          {ALL_SEVERITIES.map((s) => (
            <MenuItem key={s} value={s}>
              {SEVERITY_LABELS[s]}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={severity === currentSeverity || isSubmitting}
          onClick={() => onSubmit(severity)}
        >
          Save
        </Button>
      </Stack>
    </Dialog>
  );
}
