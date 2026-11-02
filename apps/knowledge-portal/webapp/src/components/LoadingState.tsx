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

import { Box, CircularProgress, Stack } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { VISUALLY_HIDDEN_SX } from "@components/visuallyHidden";

interface LoadingStateProps {
  /** Text announced to assistive tech (the status region is otherwise silent). */
  label?: string;
  py?: number;
}

export default function LoadingState({
  label = "Loading",
  py = 8,
}: LoadingStateProps): JSX.Element {
  return (
    <Stack alignItems="center" role="status" sx={{ py }}>
      <CircularProgress size={28} aria-hidden />
      <Box component="span" sx={VISUALLY_HIDDEN_SX}>
        {label}
      </Box>
    </Stack>
  );
}
