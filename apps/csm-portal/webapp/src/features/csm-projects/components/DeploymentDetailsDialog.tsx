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

import {
  Box,
  Button,
  Chip,
  Divider,
  Modal,
  Paper,
  Typography,
} from "@wso2/oxygen-ui";
import { type JSX, type ReactNode } from "react";
import type { BeDeployment } from "@api/backend/types";
import {
  deploymentTypeLabel,
  formatDeploymentDate,
} from "@features/csm-projects/utils/deployments";
import DeployedProductsPanel from "@features/csm-projects/components/DeployedProductsPanel";
import DeploymentAttachmentsPanel from "@features/csm-projects/components/DeploymentAttachmentsPanel";

interface DeploymentDetailsDialogProps {
  deployment: BeDeployment;
  onClose: () => void;
}

function MetaCell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

/**
 * Read-only details for a single deployment: its metadata plus the products
 * deployed in it (loaded by {@link DeployedProductsPanel} when this dialog
 * mounts). Editing the deployment and managing deployed products are separate
 * concerns handled elsewhere / not yet backed by the API. A plain `Modal` +
 * `Paper` rather than `Dialog` — same reasoning as {@link LinkCaseDialog}:
 * `Dialog`'s paper renders on the theme's more opaque `background.paper`,
 * while bare `Paper` gets the lighter, translucent "acrylic" look for free.
 */
export default function DeploymentDetailsDialog({
  deployment,
  onClose,
}: DeploymentDetailsDialogProps): JSX.Element {
  // When opened from a case, only id + name are known (no type/description/
  // dates), so the type chip and meta grid render only when there's data.
  const hasMeta = !!(
    deployment.description ||
    deployment.createdOn ||
    deployment.updatedOn
  );

  return (
    <Modal
      open
      onClose={onClose}
      slotProps={{ backdrop: { sx: { backgroundColor: "transparent" } } }}
    >
      <Paper
        elevation={3}
        role="dialog"
        aria-modal="true"
        aria-labelledby="deployment-details-dialog-title"
        sx={{
          position: "fixed",
          top: "10vh",
          left: "50%",
          transform: "translateX(-50%)",
          width: { xs: "calc(100% - 32px)", sm: "calc(100% - 64px)", md: 900 },
          maxWidth: 900,
          maxHeight: "80vh",
          display: "flex",
          flexDirection: "column",
          outline: "none",
          border: "1px solid",
          borderColor: "divider",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap", p: 3, pb: 2 }}>
          <Typography id="deployment-details-dialog-title" variant="h6">
            {deployment.name || "Deployment"}
          </Typography>
          {deployment.type && (
            <Chip size="small" variant="outlined" label={deploymentTypeLabel(deployment.type)} />
          )}
        </Box>
        <Divider />
        <Box sx={{ p: 3, overflowY: "auto", display: "flex", flexDirection: "column", gap: 2.5 }}>
          {hasMeta && (
            <Box
              sx={{
                display: "grid",
                gap: 2,
                gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))" },
              }}
            >
              {deployment.description && (
                <MetaCell label="Description">
                  <Typography variant="body2">{deployment.description}</Typography>
                </MetaCell>
              )}
              {deployment.createdOn && (
                <MetaCell label="Created">
                  <Typography variant="body2">{formatDeploymentDate(deployment.createdOn)}</Typography>
                </MetaCell>
              )}
              {deployment.updatedOn && (
                <MetaCell label="Last updated">
                  <Typography variant="body2">{formatDeploymentDate(deployment.updatedOn)}</Typography>
                </MetaCell>
              )}
            </Box>
          )}

          <DeployedProductsPanel deploymentId={deployment.id} projectId={deployment.project?.id} />

          <Divider />

          <DeploymentAttachmentsPanel deploymentId={deployment.id} />
        </Box>
        <Divider />
        <Box sx={{ display: "flex", justifyContent: "flex-end", p: 2 }}>
          <Button variant="outlined" color="inherit" onClick={onClose}>
            Close
          </Button>
        </Box>
      </Paper>
    </Modal>
  );
}
