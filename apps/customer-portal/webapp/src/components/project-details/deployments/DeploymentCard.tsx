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

import type { ProjectDeploymentItem } from "@models/responses";
import { displayValue, formatProjectDateTime } from "@utils/projectDetails";
import {
  Box,
  Card,
  CardContent,
  Chip,
  Divider,
  IconButton,
  Link,
  Typography,
} from "@wso2/oxygen-ui";
import { Calendar, PencilLine, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import DeploymentDocumentList from "@deployments/DeploymentDocumentList";
import DeploymentProductList from "@deployments/DeploymentProductList";
import EditDeploymentModal from "@deployments/EditDeploymentModal";
import DeleteDeploymentModal from "@deployments/DeleteDeploymentModal";
import { usePatchDeployment } from "@api/usePatchDeployment";

export interface DeploymentCardProps {
  deployment: ProjectDeploymentItem;
}

/**
 * Renders a single deployment environment card with products and documents.
 *
 * @param {DeploymentCardProps} props - Props containing the deployment data.
 * @returns {JSX.Element} The deployment card.
 */
export default function DeploymentCard({
  deployment,
}: DeploymentCardProps): JSX.Element {
  const { name, url, description, createdOn, updatedOn } = deployment;
  const projectId = deployment.project?.id ?? "";
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const patchDeployment = usePatchDeployment();

  const createdAtStr = formatProjectDateTime(createdOn);
  const updatedAtStr = formatProjectDateTime(updatedOn);

  return (
    <Card>
      <CardContent
        sx={{ p: 3, display: "flex", flexDirection: "column", gap: 3 }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 3,
            flex: 1,
          }}
        >
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1.5,
                mb: 0.5,
                flexWrap: "wrap",
              }}
            >
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {displayValue(name, "Not Available")}
              </Typography>
              {deployment.type?.label && (
                <Chip
                  label={deployment.type.label}
                  size="small"
                  variant="outlined"
                  sx={{ height: 20, fontSize: "0.75rem" }}
                />
              )}
            </Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              {url ? (
                <Link
                  href={url}
                  target="_blank"
                  rel="noopener noreferrer"
                  variant="body2"
                  sx={{ color: "text.secondary" }}
                >
                  {url}
                </Link>
              ) : (
                <Typography variant="body2" color="text.secondary">
                  {displayValue(url, "Not Available")}
                </Typography>
              )}
            </Box>
          </Box>
          <Box sx={{ display: "flex", gap: 0.25, alignItems: "center" }}>
            <IconButton
              component="div"
              size="small"
              role="button"
              aria-label="Edit deployment"
              onClick={() => setIsEditModalOpen(true)}
              sx={{
                color: "text.secondary",
                "&:hover": { color: "primary.main" },
                "&.Mui-focusVisible": { color: "primary.main" },
              }}
            >
              <PencilLine size={16} aria-hidden />
            </IconButton>
            <IconButton
              component="div"
              size="small"
              role="button"
              aria-label="Delete deployment"
              onClick={() => setIsDeleteModalOpen(true)}
              disabled={patchDeployment.isPending}
              sx={{
                color: "text.secondary",
                "&:hover": { color: "primary.main" },
                "&.Mui-focusVisible": { color: "primary.main" },
              }}
            >
              <Trash2 size={16} aria-hidden />
            </IconButton>
          </Box>
        </Box>

        <Divider />
        <Typography variant="body2" color="text.secondary">
          {displayValue(description, "Not Available")}
        </Typography>
        <Divider />

        <DeploymentProductList
          deploymentId={deployment.id}
          projectId={deployment.project?.id ?? ""}
        />

        <Divider />

        <DeploymentDocumentList deploymentId={deployment.id} />

        <Divider />
        <Box
          sx={{
            display: "inline-flex",
            alignItems: "center",
            gap: 0.5,
            color: "text.secondary",
            flexShrink: 0,
            fontSize: "0.75rem",
          }}
        >
          <Calendar
            size={14}
            style={{
              verticalAlign: "middle",
              display: "inline-block",
              marginTop: "-2px",
            }}
          />
          <span style={{ verticalAlign: "middle", whiteSpace: "nowrap" }}>
            Created on {createdAtStr} • Updated on {updatedAtStr}
          </span>
        </Box>
      </CardContent>

      <EditDeploymentModal
        open={isEditModalOpen}
        deployment={deployment}
        projectId={projectId}
        onClose={() => setIsEditModalOpen(false)}
        onSuccess={() => setIsEditModalOpen(false)}
      />

      <DeleteDeploymentModal
        open={isDeleteModalOpen}
        deployment={deployment}
        onClose={() => setIsDeleteModalOpen(false)}
        isDeleting={patchDeployment.isPending}
        onConfirm={() => {
          patchDeployment.mutate(
            {
              projectId,
              deploymentId: deployment.id,
              body: { active: false },
            },
            {
              onSuccess: () => setIsDeleteModalOpen(false),
            },
          );
        }}
      />
    </Card>
  );
}
