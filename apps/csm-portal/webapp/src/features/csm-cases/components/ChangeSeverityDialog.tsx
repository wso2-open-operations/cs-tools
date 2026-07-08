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
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  Radio,
  RadioGroup,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";

const SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];

/** S0-S3 are the on-call-paging severities (matches the "High Severity Case
 * Alert" threshold everywhere else in the portal this distinction shows up —
 * e.g. the E2E suite deliberately only ever creates S4 test cases to avoid
 * triggering it). S4 is the only severity that never pages anyone. */
function pagesOnCall(severity: Severity): boolean {
  return severity !== "S4";
}

interface ChangeSeverityDialogProps {
  currentSeverity: Severity;
  /** True while a PATCH is in flight; disables the actions. */
  isChanging: boolean;
  onClose: () => void;
  /** Apply the new severity (`PATCH { severity }`). */
  onChange: (next: Severity) => void;
}

/**
 * Change a case's severity via `PATCH /cases/{id}` (`severity`). The backend
 * and the search/filter UI already fully support this — see
 * usePatchCsmCase.ts's own doc comment ("state transitions, priority
 * changes, ...") and CaseAuditKind's `severity_change` entry, which the
 * activity feed already knows how to render — this dialog was the only
 * missing piece. Raising to S0-S3 can page on-call staff, so that's called
 * out before confirming rather than left to be discovered after the fact.
 */
export default function ChangeSeverityDialog({
  currentSeverity,
  isChanging,
  onClose,
  onChange,
}: ChangeSeverityDialogProps): JSX.Element {
  const [selected, setSelected] = useState<Severity>(currentSeverity);

  const changed = selected !== currentSeverity;
  const willPage = changed && pagesOnCall(selected) && !pagesOnCall(currentSeverity);

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Change severity</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          Current severity:{" "}
          <Typography component="span" variant="body2" sx={{ fontWeight: 600, color: "text.primary" }}>
            {currentSeverity} · {SEVERITY_LABEL[currentSeverity]}
          </Typography>
        </Typography>

        <FormControl>
          <RadioGroup
            value={selected}
            onChange={(e) => setSelected(e.target.value as Severity)}
          >
            {SEVERITIES.map((s) => (
              <FormControlLabel
                key={s}
                value={s}
                disabled={isChanging}
                control={<Radio size="small" />}
                label={`${s} · ${SEVERITY_LABEL[s]}`}
              />
            ))}
          </RadioGroup>
        </FormControl>

        {willPage && (
          <Alert severity="warning" sx={{ mt: 1.5 }}>
            Raising to {selected} may page on-call staff.
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isChanging}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color={willPage ? "warning" : "primary"}
          disabled={!changed || isChanging}
          loading={isChanging}
          onClick={() => onChange(selected)}
        >
          Change severity
        </Button>
      </DialogActions>
    </Dialog>
  );
}
