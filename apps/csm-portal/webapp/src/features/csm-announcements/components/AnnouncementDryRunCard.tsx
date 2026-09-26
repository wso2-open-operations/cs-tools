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

import { Box, Button, Card, Typography } from "@wso2/oxygen-ui";
import { FlaskConical } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link } from "react-router";
import { DRY_RUN_TEST_PROJECT_KEY } from "@config/announcementDryRunConfig";
import type { DryRunResult } from "@features/csm-announcements/api/useAnnouncementDryRun";

interface AnnouncementDryRunCardProps {
  runningDryRun: boolean;
  dryRunResult: DryRunResult | null;
  canRunDryRun: boolean;
  onRunDryRun: () => void;
}

/**
 * The dry-run section every announcement create flow renders — same
 * prominent, warning-bordered Card, same copy, same button and result link,
 * regardless of which flow (Option 1 or Option 2) is using it. Given its own
 * prominent section (not folded into the Cancel/Create row) because the
 * source process treats it as the mandatory first step before any real send,
 * not an optional afterthought. See useAnnouncementDryRun for the mechanism
 * this renders the state of.
 */
export default function AnnouncementDryRunCard({
  runningDryRun,
  dryRunResult,
  canRunDryRun,
  onRunDryRun,
}: AnnouncementDryRunCardProps): JSX.Element {
  return (
    <Card
      variant="outlined"
      sx={{
        mt: 2.5,
        p: 2.5,
        bgcolor: "action.hover",
        borderColor: "warning.main",
        display: "flex",
        flexDirection: "column",
        gap: 1.25,
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <FlaskConical size={18} />
        <Typography variant="subtitle1" fontWeight={700}>
          Dry run
        </Typography>
        <Typography variant="caption" color="text.secondary">
          (do this before sending to customers)
        </Typography>
      </Box>
      <Typography variant="body2" color="text.secondary">
        Creates one real case in the <strong>{DRY_RUN_TEST_PROJECT_KEY}</strong> test project with
        this exact subject, description, and label, so you can open it and check formatting
        before it goes out to real customer projects.
      </Typography>
      <Box>
        <Button
          variant="contained"
          color="warning"
          startIcon={<FlaskConical size={16} />}
          onClick={onRunDryRun}
          disabled={!canRunDryRun}
        >
          {runningDryRun ? "Running dry run…" : "Run dry run"}
        </Button>
      </Box>
      {dryRunResult && (
        <Typography variant="body2" color="success.main">
          Dry run case created ({dryRunResult.displayId}) —{" "}
          <Link to={`/announcements/${dryRunResult.caseId}`} target="_blank" rel="noopener noreferrer">
            view it
          </Link>
          .
        </Typography>
      )}
    </Card>
  );
}
