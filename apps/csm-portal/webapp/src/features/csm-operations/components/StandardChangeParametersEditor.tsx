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
  Checkbox,
  FormControlLabel,
  IconButton,
  Paper,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowDown, ArrowUp, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import {
  MAX_STANDARD_CHANGE_PARAMETERS,
  STANDARD_CHANGE_PARAMETER_TYPE_OPTIONS,
  type ParameterIssues,
} from "@features/csm-operations/utils/standardChanges";
import type { BeStandardChangeParameter, BeStandardChangeParameterType } from "@api/backend/types";

interface StandardChangeParametersEditorProps {
  parameters: BeStandardChangeParameter[];
  onChange: (next: BeStandardChangeParameter[]) => void;
  /** One entry per parameter (see `validateStandardChangeParameters`). */
  issues: readonly ParameterIssues[];
  disabled?: boolean;
}

/** A new row's key: `param_N`, the first N not already taken. */
function nextKey(parameters: readonly BeStandardChangeParameter[]): string {
  const taken = new Set(parameters.map((p) => p.key));
  let n = parameters.length + 1;
  while (taken.has(`param_${n}`)) n += 1;
  return `param_${n}`;
}

/**
 * The parameters a standard change template leaves open (its "change
 * specifics"): each a key the text refers to as `{{key}}`, a label, a type,
 * whether it is required, help text and, for a choice, its options (one per
 * line). Rows can be added (up to 30), removed and moved up or down: the order
 * is the order of the inputs on the create form.
 */
export default function StandardChangeParametersEditor({
  parameters,
  onChange,
  issues,
  disabled = false,
}: StandardChangeParametersEditorProps): JSX.Element {
  const update = (i: number, patch: Partial<BeStandardChangeParameter>): void => {
    onChange(parameters.map((p, at) => (at === i ? { ...p, ...patch } : p)));
  };
  const move = (i: number, by: -1 | 1): void => {
    const j = i + by;
    if (j < 0 || j >= parameters.length) return;
    const next = [...parameters];
    [next[i], next[j]] = [next[j]!, next[i]!];
    onChange(next);
  };
  const atMax = parameters.length >= MAX_STANDARD_CHANGE_PARAMETERS;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      {parameters.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          No parameters yet. Add one for each value that differs from one change to the next (a host name, a
          version, a date), then refer to it in the text as {"{{key}}"}.
        </Typography>
      )}
      {parameters.map((p, i) => {
        const rowIssues = issues[i] ?? {};
        const name = p.label.trim() || p.key || `parameter ${i + 1}`;
        return (
          <Paper
            key={i}
            variant="outlined"
            role="group"
            aria-label={`Parameter ${i + 1}`}
            sx={{ p: 1.5, display: "flex", flexDirection: "column", gap: 1.5 }}
          >
            <Box
              sx={{
                display: "grid",
                gap: 1.5,
                gridTemplateColumns: { xs: "1fr", md: "minmax(0, 1fr) minmax(0, 1.4fr) minmax(0, 1fr) auto" },
                alignItems: "start",
              }}
            >
              <TextField
                label="Key"
                size="small"
                value={p.key}
                disabled={disabled}
                onChange={(e) => update(i, { key: e.target.value })}
                error={!!rowIssues.key}
                helperText={rowIssues.key ?? `Used as {{${p.key || "key"}}}`}
                slotProps={{ htmlInput: { "aria-label": `Key of parameter ${i + 1}`, spellCheck: false } }}
              />
              <TextField
                label="Label"
                size="small"
                value={p.label}
                disabled={disabled}
                onChange={(e) => update(i, { label: e.target.value })}
                error={!!rowIssues.label}
                helperText={rowIssues.label}
                slotProps={{ htmlInput: { "aria-label": `Label of parameter ${i + 1}` } }}
              />
              <TextField
                select
                label="Type"
                size="small"
                value={p.type}
                disabled={disabled}
                onChange={(e) => {
                  const type = e.target.value as BeStandardChangeParameterType;
                  update(i, { type, options: type === "select" ? (p.options ?? []) : undefined });
                }}
                slotProps={{
                  select: { native: true },
                  inputLabel: { shrink: true },
                  htmlInput: { "aria-label": `Type of parameter ${i + 1}` },
                }}
              >
                {STANDARD_CHANGE_PARAMETER_TYPE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </TextField>
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                <FormControlLabel
                  sx={{ m: 0 }}
                  disabled={disabled}
                  control={
                    <Checkbox
                      size="small"
                      checked={p.required}
                      onChange={(e) => update(i, { required: e.target.checked })}
                      inputProps={{ "aria-label": `Parameter ${i + 1} is required` }}
                    />
                  }
                  label="Required"
                />
                <Tooltip title="Move up">
                  <span>
                    <IconButton
                      size="small"
                      aria-label={`Move ${name} up`}
                      disabled={disabled || i === 0}
                      onClick={() => move(i, -1)}
                    >
                      <ArrowUp size={16} />
                    </IconButton>
                  </span>
                </Tooltip>
                <Tooltip title="Move down">
                  <span>
                    <IconButton
                      size="small"
                      aria-label={`Move ${name} down`}
                      disabled={disabled || i === parameters.length - 1}
                      onClick={() => move(i, 1)}
                    >
                      <ArrowDown size={16} />
                    </IconButton>
                  </span>
                </Tooltip>
                <Tooltip title="Remove">
                  <span>
                    <IconButton
                      size="small"
                      color="error"
                      aria-label={`Remove ${name}`}
                      disabled={disabled}
                      onClick={() => onChange(parameters.filter((_, at) => at !== i))}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </span>
                </Tooltip>
              </Box>
            </Box>
            <TextField
              label="Help text"
              size="small"
              value={p.helpText ?? ""}
              disabled={disabled}
              onChange={(e) => update(i, { helpText: e.target.value })}
              error={!!rowIssues.helpText}
              helperText={rowIssues.helpText ?? "Shown under the input when the change request is raised."}
              slotProps={{ htmlInput: { "aria-label": `Help text of parameter ${i + 1}` } }}
            />
            {p.type === "select" && (
              <TextField
                label="Options"
                size="small"
                multiline
                minRows={2}
                // One option per line; kept as typed (blank lines included) while editing, trimmed when sent.
                value={(p.options ?? []).join("\n")}
                disabled={disabled}
                onChange={(e) => update(i, { options: e.target.value.split("\n") })}
                error={!!rowIssues.options}
                helperText={rowIssues.options ?? "One option per line, in the order they are offered."}
                slotProps={{ htmlInput: { "aria-label": `Options of parameter ${i + 1}` } }}
              />
            )}
          </Paper>
        );
      })}
      <Box>
        <Button
          size="small"
          variant="outlined"
          startIcon={<Plus size={16} />}
          disabled={disabled || atMax}
          onClick={() =>
            onChange([...parameters, { key: nextKey(parameters), label: "", type: "text", required: true }])
          }
        >
          Add parameter
        </Button>
        {atMax && (
          <Typography variant="caption" color="text.secondary" sx={{ ml: 1 }}>
            A template can declare at most {MAX_STANDARD_CHANGE_PARAMETERS} parameters.
          </Typography>
        )}
      </Box>
    </Box>
  );
}
