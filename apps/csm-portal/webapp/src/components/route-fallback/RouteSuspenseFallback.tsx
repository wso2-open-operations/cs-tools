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

import { Box, LinearProgress } from "@wso2/oxygen-ui";
import { type JSX } from "react";

/**
 * Fallback rendered while a lazily-loaded route chunk is being fetched. Sits
 * inside the persistent app shell so only the content region shows progress,
 * mirroring the primary-coloured LinearProgress used elsewhere in the layout.
 */
export default function RouteSuspenseFallback(): JSX.Element {
  return (
    <Box
      sx={{
        flex: 1,
        minHeight: 0,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: 2,
      }}
    >
      <LinearProgress
        color="primary"
        sx={{ width: "80%", maxWidth: 400, height: 4 }}
      />
    </Box>
  );
}
