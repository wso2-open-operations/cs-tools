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
import { Button, colors, LinearProgress, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { Pin } from "@wso2/oxygen-ui-icons-react";

export function PromptCreateCase({ onCreateCase, pending = false }: { onCreateCase: () => void; pending?: boolean }) {
  return (
    <Stack sx={{ borderBottom: `1px solid divider`, position: "relative" }}>
      {pending && (
        <LinearProgress color="inherit" sx={{ position: "absolute", top: 0, left: 0, right: 0, height: 2 }} />
      )}

      <Stack direction="row" alignItems="center" gap={2} p={2}>
        <Pin
          size={pxToRem(12)}
          fill={colors.grey[500]}
          style={{ color: colors.grey[500], position: "absolute", right: 3, top: 5 }}
        />

        <Typography variant="body2" sx={{ opacity: pending ? 0.3 : 1 }}>
          I can create a support case with all the details we've discussed.
        </Typography>

        <Button
          variant="outlined"
          sx={{ textTransform: "initial", flexShrink: 0 }}
          onClick={onCreateCase}
          disabled={pending}
        >
          Create Case
        </Button>
      </Stack>
    </Stack>
  );
}
