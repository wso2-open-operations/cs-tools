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

import { alpha, Chip, type ChipProps } from "@wso2/oxygen-ui";
import type { CaseSeverity, CaseState } from "@src/types";
import { SEVERITY_CHIP_COLOR_CONFIG, SEVERITY_LABELS, STATE_CHIP_COLOR_CONFIG, STATE_LABELS } from "./config";

function ColoredChip({
  color,
  label,
  ...props
}: Omit<ChipProps, "label" | "color"> & { color: NonNullable<ChipProps["color"]>; label: string }) {
  return (
    <Chip
      label={label}
      size="small"
      {...props}
      sx={
        color !== "default"
          ? (theme) => ({ bgcolor: alpha(theme.palette[color].light, 0.1), color: theme.palette[color].light })
          : undefined
      }
    />
  );
}

export function StatusChip({ state, ...props }: { state: CaseState } & Omit<ChipProps, "label" | "color">) {
  return (
    <ColoredChip color={STATE_CHIP_COLOR_CONFIG[state] ?? "default"} label={STATE_LABELS[state] ?? state} {...props} />
  );
}

export function SeverityChip({ severity, ...props }: { severity: CaseSeverity } & Omit<ChipProps, "label" | "color">) {
  return (
    <ColoredChip
      color={SEVERITY_CHIP_COLOR_CONFIG[severity] ?? "default"}
      label={SEVERITY_LABELS[severity] ?? severity}
      {...props}
    />
  );
}
