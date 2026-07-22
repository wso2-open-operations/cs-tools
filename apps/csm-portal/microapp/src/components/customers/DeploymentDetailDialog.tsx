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

import { Chip, Dialog, DialogContent, DialogTitle, Divider, IconButton, Stack, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { Deployment } from "@src/types";
import { formatDateOnly, formatEnumLabel } from "@utils/customers";
import { DialogPaper } from "@components/common/DialogPaper";
import { MetaRow, MetaValue } from "@components/common/MetaRow";
import { DeployedProductsList } from "@components/customers/DeployedProductsList";

interface DeploymentDetailDialogProps {
  deployment: Deployment | null;
  onClose: () => void;
}

// Read-only deployment detail (name/type/description/dates) + its Deployed Products list —
// the mobile dialog equivalent of the webapp's DeploymentDetailsDialog, with
// DeployedProductsPanel folded in underneath rather than needing its own expand/click.
export function DeploymentDetailDialog({ deployment, onClose }: DeploymentDetailDialogProps) {
  return (
    <Dialog open={!!deployment} onClose={onClose} fullWidth maxWidth="xs" slots={{ paper: DialogPaper }}>
      <DialogTitle sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        {deployment?.name || "Deployment"}
        <IconButton size="small" aria-label="Close" onClick={onClose}>
          <X size={18} />
        </IconButton>
      </DialogTitle>

      <Divider />

      {deployment && (
        <DialogContent>
          <Stack gap={2}>
            <Stack gap={1.5}>
              <MetaRow label="Type">
                <Chip size="small" variant="outlined" label={formatEnumLabel(deployment.type)} />
              </MetaRow>
              <Divider />
              <MetaRow label="Description">
                <MetaValue>{deployment.description || "—"}</MetaValue>
              </MetaRow>
              <Divider />
              <MetaRow label="Created">
                <MetaValue>{formatDateOnly(deployment.createdOn)}</MetaValue>
              </MetaRow>
              <Divider />
              <MetaRow label="Updated">
                <MetaValue>{formatDateOnly(deployment.updatedOn)}</MetaValue>
              </MetaRow>
            </Stack>

            <Stack gap={1}>
              <Typography variant="subtitle2">Deployed products</Typography>
              <DeployedProductsList deploymentId={deployment.id} />
            </Stack>
          </Stack>
        </DialogContent>
      )}
    </Dialog>
  );
}
