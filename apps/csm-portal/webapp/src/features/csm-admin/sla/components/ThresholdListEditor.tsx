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

import { Box, Chip, TextField, Tooltip } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";

interface Props {
  value: number[];
  onChange: (next: number[]) => void;
  disabled?: boolean;
}

/** A definition's threshold list (R39): percentages of the target, always
 *  including 100 (the breach threshold, never removable here). */
export default function ThresholdListEditor({ value, onChange, disabled }: Props): JSX.Element {
  const [draft, setDraft] = useState("");

  const sorted = [...value].sort((a, b) => a - b);

  const addDraft = (): void => {
    const n = Number(draft);
    // Must match entity-service's own bounds exactly (integers, 1-1000):
    // a fractional or out-of-range value would otherwise pass this check
    // and only fail on save, as a generic 400 that doesn't say which
    // threshold caused it.
    if (!Number.isInteger(n) || n < 1 || n > 1000 || value.includes(n)) return;
    onChange([...value, n]);
    setDraft("");
  };

  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
      {sorted.map((p) =>
        p === 100 ? (
          <Tooltip key={p} title="The breach threshold — always included">
            <Chip label="100%" color="warning" />
          </Tooltip>
        ) : (
          <Chip
            key={p}
            label={`${p}%`}
            onDelete={disabled ? undefined : () => onChange(value.filter((v) => v !== p))}
          />
        ),
      )}
      <TextField
        size="small"
        type="number"
        placeholder="e.g. 75"
        value={draft}
        disabled={disabled}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            addDraft();
          }
        }}
        sx={{ width: 110 }}
      />
      <Chip
        icon={<Plus size={14} />}
        label="Add"
        variant="outlined"
        onClick={disabled ? undefined : addDraft}
        disabled={disabled || !draft}
      />
    </Box>
  );
}
