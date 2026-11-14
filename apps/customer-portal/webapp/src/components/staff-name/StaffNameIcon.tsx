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

import { Box } from "@wso2/oxygen-ui";
import { Headset } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

/**
 * Marks a name as a support engineer's. Shown after the name once the circled
 * W the data carries has been stripped (see `splitStaffMark`).
 *
 * @returns {JSX.Element} The icon.
 */
export default function StaffNameIcon({
  size = 16,
}: {
  size?: number;
}): JSX.Element {
  return (
    <Box
      component="span"
      role="img"
      aria-label="Support engineer"
      sx={{
        display: "inline-flex",
        alignItems: "center",
        verticalAlign: "middle",
        ml: 0.5,
        color: "info.main",
      }}
    >
      <Headset size={size} />
    </Box>
  );
}
