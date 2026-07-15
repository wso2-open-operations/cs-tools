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

import { Button, Stack, Typography } from "@wso2/oxygen-ui";
import { useNavigate } from "react-router-dom";

/**
 * The time-cards search endpoint has no working case-scoping (see services/timecards.ts —
 * `caseId` is "non-functional live"), so this can't show a genuine per-case time list yet. Rather
 * than fake a filtered view, this links out to the full Time Cards page instead.
 */
export function TimeTrackingTab({ caseNumber }: { caseNumber: string }) {
  const navigate = useNavigate();

  return (
    <Stack gap={1.5} alignItems="start">
      <Typography variant="body2" color="text.secondary">
        Per-case time tracking isn&apos;t available here yet. Log or review time for {caseNumber} from the full Time
        Cards page.
      </Typography>
      <Button variant="outlined" size="small" onClick={() => navigate("/more/time-cards")}>
        Open Time Cards
      </Button>
    </Stack>
  );
}
