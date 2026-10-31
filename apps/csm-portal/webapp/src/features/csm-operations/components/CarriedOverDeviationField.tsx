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

import { Box, Button } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import ExpandableRichTextField from "@features/csm-operations/components/ExpandableRichTextField";
import { IMPLEMENTATION_DEVIATION_CARRIED_OVER_REASON } from "@features/csm-operations/utils/changeRequests";

interface CarriedOverDeviationFieldProps {
  /** The field's id (the label and helper ids derive from it). */
  id: string;
  /** The deviation the clone carried over, shown read-only. */
  html: string;
  /** Whether the user chose to clear it. */
  cleared: boolean;
  onClearedChange: (cleared: boolean) => void;
  /** What happens to it once cleared, e.g. that the save clears it. */
  clearedText: string;
  disabled?: boolean;
  sx?: Record<string, unknown>;
}

/**
 * The deviation from the implementation plan a clone carried over, before
 * implementation: shown read-only (it is recorded from Implement on), with what
 * to do about it -- fold it into the implementation plan, then clear it, which
 * Request Approval needs -- and a "Clear deviation" action. Clearing is a choice
 * the form sends with its save (so the plan can be edited and the deviation
 * cleared in one save, or in two), and "Keep deviation" undoes it. The text stays
 * on screen either way, so it can still be copied into the plan.
 */
export default function CarriedOverDeviationField({
  id,
  html,
  cleared,
  onClearedChange,
  clearedText,
  disabled = false,
  sx,
}: CarriedOverDeviationFieldProps): JSX.Element {
  return (
    <Box sx={sx}>
      <ExpandableRichTextField
        id={id}
        label="Deviation from the implementation plan"
        value={html}
        // Read-only: the only change offered is clearing it, below.
        onChange={() => undefined}
        disabled
        helperText={`${cleared ? clearedText : IMPLEMENTATION_DEVIATION_CARRIED_OVER_REASON} Internal: never shown to the customer.`}
      />
      <Button
        size="small"
        variant="outlined"
        disabled={disabled}
        onClick={() => onClearedChange(!cleared)}
        sx={{ mt: 1 }}
      >
        {cleared ? "Keep deviation" : "Clear deviation"}
      </Button>
    </Box>
  );
}
