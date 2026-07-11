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
import type { TimeCardState, TimeSheetState } from "@src/types";
import { TIME_CARD_STATE_META, TIME_SHEET_STATE_META } from "@utils/timecard";

// Mirrors the support ColoredChip look: a soft alpha-tinted background for a
// coloured state, plain default otherwise.
function StateChip({ label, color }: { label: string; color: ChipProps["color"] }) {
  return (
    <Chip
      label={label}
      size="small"
      sx={
        color && color !== "default"
          ? (theme) => ({ bgcolor: alpha(theme.palette[color].light, 0.1), color: theme.palette[color].light })
          : undefined
      }
    />
  );
}

export function TimeCardStateChip({ state }: { state: TimeCardState }) {
  const meta = TIME_CARD_STATE_META[state];
  return <StateChip label={meta?.label ?? state} color={meta?.color ?? "default"} />;
}

export function TimeSheetStateChip({ state }: { state: TimeSheetState }) {
  const meta = TIME_SHEET_STATE_META[state];
  return <StateChip label={meta?.label ?? state} color={meta?.color ?? "default"} />;
}
