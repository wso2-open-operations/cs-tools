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

import { Box, Header as HeaderUI, type SxProps, type Theme } from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";

export interface HeaderSwitchersSlotProps {
  children: ReactNode;
  stackedHeaderRow?: boolean;
  sx?: SxProps<Theme>;
}

/**
 * Wraps header switcher content in Oxygen Header.Switchers or a plain Box for inline rows.
 *
 * @param {HeaderSwitchersSlotProps} props - Slot children and layout mode.
 * @returns {JSX.Element} Wrapped switcher region.
 */
export default function HeaderSwitchersSlot({
  children,
  stackedHeaderRow = false,
  sx,
}: HeaderSwitchersSlotProps): JSX.Element {
  if (stackedHeaderRow) {
    return (
      <Box
        sx={[
          {
            display: "flex",
            alignItems: "center",
            minWidth: 0,
            flex: "1 1 auto",
            width: "100%",
            maxWidth: "100%",
          },
          ...(Array.isArray(sx) ? sx : [sx]),
        ]}
      >
        {children}
      </Box>
    );
  }

  return (
    <HeaderUI.Switchers showDivider={false} sx={sx}>
      {children}
    </HeaderUI.Switchers>
  );
}
