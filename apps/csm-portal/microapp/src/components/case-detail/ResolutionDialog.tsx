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
import {
  Button,
  Dialog,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import type { CaseCause, CaseResolutionCode } from "@src/types";
import { CASE_CAUSES, humanizeEnumValue, RESOLUTION_CODES } from "./caseResolution";
import { DialogPaper } from "@components/common/DialogPaper";

interface ResolutionDialogProps {
  /** Which lifecycle transition this resolution is being collected for — only used for the title. */
  kind: "closed" | "solution_proposed";
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (fields: { resolutionCode: CaseResolutionCode; cause: CaseCause; closeNotes: string }) => void;
}

const KIND_TITLE: Record<ResolutionDialogProps["kind"], string> = {
  closed: "Close case",
  solution_proposed: "Propose solution",
};

/** Collects the resolution code, root cause, and close notes the backend optionally accepts
 * alongside `state: closed` / `state: solution_proposed` — mirrors the webapp's ResolutionDialog
 * (ISSU-026). Unlike the webapp, submitting without a resolution isn't blocked by the backend, but
 * a lead needs this data to actually know why a case closed, so both fields stay required here. */
export function ResolutionDialog({ kind, isSubmitting, onClose, onSubmit }: ResolutionDialogProps) {
  const [resolutionCode, setResolutionCode] = useState<CaseResolutionCode | "">("");
  const [cause, setCause] = useState<CaseCause | "">("");
  const [closeNotes, setCloseNotes] = useState("");

  const canSubmit = !!resolutionCode && !!cause && !isSubmitting;

  return (
    <Dialog
      open
      onClose={onClose}
      slots={{ paper: DialogPaper }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        {KIND_TITLE[kind]}
      </Typography>

      <FormControl size="small" fullWidth>
        <InputLabel id="resolution-code-label">Resolution code</InputLabel>
        <Select
          labelId="resolution-code-label"
          label="Resolution code"
          value={resolutionCode}
          onChange={(e) => setResolutionCode(e.target.value as CaseResolutionCode)}
        >
          {RESOLUTION_CODES.map((code) => (
            <MenuItem key={code} value={code}>
              {humanizeEnumValue(code)}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      <FormControl size="small" fullWidth>
        <InputLabel id="case-cause-label">Cause</InputLabel>
        <Select
          labelId="case-cause-label"
          label="Cause"
          value={cause}
          onChange={(e) => setCause(e.target.value as CaseCause)}
        >
          {CASE_CAUSES.map((c) => (
            <MenuItem key={c} value={c}>
              {humanizeEnumValue(c)}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      <TextField
        label="Close notes"
        multiline
        minRows={3}
        fullWidth
        size="small"
        value={closeNotes}
        onChange={(e) => setCloseNotes(e.target.value)}
      />

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canSubmit}
          onClick={() => {
            if (!resolutionCode || !cause) return;
            onSubmit({ resolutionCode, cause, closeNotes });
          }}
        >
          Submit
        </Button>
      </Stack>
    </Dialog>
  );
}
