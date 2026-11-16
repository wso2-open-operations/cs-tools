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
import type {
  Severity,
  SeverityOrUnset,
} from "@features/csm-dashboard/types/abtDashboard";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";

const SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
/** A service request never holds S0: its severity is S1-S4 only. */
const SERVICE_REQUEST_SEVERITIES: Severity[] = ["S1", "S2", "S3", "S4"];

interface ChangeSeverityDialogProps {
  /**
   * May be `"unset"` when the case has no severity value at all — the radio
   * group then starts with nothing selected (none of `SEVERITIES` match),
   * which correctly forces the engineer to pick a real value before "Change
   * severity" enables.
   */
  currentSeverity: SeverityOrUnset;
  /** True when the case's project is a Managed Cloud subscription — S0 is
   * reserved for those, same rule as case creation (see CsmCaseCreatePage.tsx). */
  isManagedCloud: boolean;
  /** True when the record is a service request: only S1-S4 are offered (no S0
   * option at all, rather than a disabled one), and the severity is internal. */
  isServiceRequest?: boolean;
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
 * missing piece.
 */
export default function ChangeSeverityDialog({
  currentSeverity,
  isManagedCloud,
  isServiceRequest = false,
  isChanging,
  onClose,
  onChange,
}: ChangeSeverityDialogProps): JSX.Element {
  const [selected, setSelected] = useState<SeverityOrUnset>(currentSeverity);

  const changed = selected !== currentSeverity;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Change severity</DialogTitle>
      <DialogContent>
        {isServiceRequest && (
          <Typography variant="caption" color="text.secondary" component="p" sx={{ mb: 1 }}>
            A service request&rsquo;s severity is internal and is never shown to the customer.
          </Typography>
        )}
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          Current severity:{" "}
          <Typography component="span" variant="body2" sx={{ fontWeight: 600, color: "text.primary" }}>
            {currentSeverity === "unset"
              ? "Not set"
              : `${currentSeverity} · ${SEVERITY_LABEL[currentSeverity]}`}
          </Typography>
        </Typography>

        <FormControl>
          <RadioGroup
            value={selected}
            onChange={(e) => setSelected(e.target.value as Severity)}
          >
            {(isServiceRequest ? SERVICE_REQUEST_SEVERITIES : SEVERITIES).map((s) => {
              const s0Blocked = s === "S0" && !isManagedCloud;
              return (
                <FormControlLabel
                  key={s}
                  value={s}
                  disabled={isChanging || s0Blocked}
                  control={<Radio size="small" />}
                  label={
                    s0Blocked
                      ? `${s} · ${SEVERITY_LABEL[s]} (Managed Cloud only)`
                      : `${s} · ${SEVERITY_LABEL[s]}`
                  }
                />
              );
            })}
          </RadioGroup>
        </FormControl>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isChanging}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!changed || isChanging || selected === "unset"}
          loading={isChanging}
          onClick={() => {
            // Guard is redundant with `disabled` above (the radio group never
            // offers "unset" as an option) but keeps this call type-safe.
            if (selected !== "unset") onChange(selected);
          }}
        >
          Change severity
        </Button>
      </DialogActions>
    </Dialog>
  );
}
