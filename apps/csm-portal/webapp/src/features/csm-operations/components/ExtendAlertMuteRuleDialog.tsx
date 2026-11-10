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
  AdapterDateFns,
  Alert,
  Box,
  Button,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { AlertMuteRule } from "@features/csm-operations/types/alertMuteRules";
import { formatDuration, latestAllowedEnd, MAX_ALERT_MUTE_RULE_HOURS } from "@features/csm-operations/utils/alertMuteRules";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

const { DateTimePicker, LocalizationProvider } = DatePickers;

const QUICK_EXTENSIONS_MIN = [30, 60, 120];

interface ExtendAlertMuteRuleDialogProps {
  rule: AlertMuteRule;
  isSubmitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (endOn: string) => void;
}

/**
 * Pushes a rule's end out: by 30 min / 1 h / 2 h, or to a picked time. The
 * rule still can't run more than {@link MAX_ALERT_MUTE_RULE_HOURS} hours from its
 * start, so the picker stops there and a quick button that would pass it is
 * disabled.
 */
export default function ExtendAlertMuteRuleDialog({
  rule,
  isSubmitting,
  error,
  onClose,
  onSubmit,
}: ExtendAlertMuteRuleDialogProps): JSX.Element {
  const currentEnd = Date.parse(rule.endOn);
  const latest = latestAllowedEnd(rule);
  const [end, setEnd] = useState<Date | null>(null);

  const endMs = end && !Number.isNaN(end.getTime()) ? end.getTime() : null;
  const invalid =
    endMs === null
      ? null
      : endMs <= currentEnd
        ? "Pick a time after the current end."
        : endMs > latest
          ? `A rule can run for at most ${MAX_ALERT_MUTE_RULE_HOURS} hours from its start.`
          : null;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth aria-labelledby="extend-alert-mute-rule-title">
      <DialogTitle id="extend-alert-mute-rule-title">Extend alert mute rule</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {error && (
            <Alert severity="error" role="alert">
              {error}
            </Alert>
          )}
          <Typography variant="body2" color="text.secondary">
            {`Currently ends ${formatBackendTimestampForDisplay(rule.endOn, { dateStyle: "medium", timeStyle: "short" }) ?? "—"}.`}
          </Typography>
          <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
            {QUICK_EXTENSIONS_MIN.map((min) => {
              const target = currentEnd + min * 60_000;
              return (
                <Button
                  key={min}
                  variant={endMs === target ? "contained" : "outlined"}
                  size="small"
                  disabled={isSubmitting || target > latest}
                  onClick={() => setEnd(new Date(target))}
                >
                  {`+${formatDuration(min * 60_000)}`}
                </Button>
              );
            })}
          </Box>
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label="New end"
              value={end}
              disabled={isSubmitting}
              minDateTime={new Date(currentEnd)}
              maxDateTime={new Date(latest)}
              onChange={(next: unknown) => setEnd(next instanceof Date ? next : null)}
              slotProps={{
                textField: { size: "small", fullWidth: true, required: true, error: !!invalid, helperText: invalid ?? undefined },
              }}
            />
          </LocalizationProvider>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={isSubmitting || endMs === null || !!invalid}
          loading={isSubmitting}
          onClick={() => endMs !== null && onSubmit(new Date(endMs).toISOString())}
        >
          Extend
        </Button>
      </DialogActions>
    </Dialog>
  );
}
