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

import type { DeploymentCardProps } from "@features/project-details/types/projectDetailsComponents";
import {
  displayValue,
  formatProjectDateTime,
} from "@features/project-details/utils/projectDetails";
import {
  Box,
  Card,
  CardContent,
  Chip,
  Divider,
  Link,
  Typography,
} from "@wso2/oxygen-ui";
import DeploymentCardLicenseFooter from "@features/project-details/components/deployments/deployment-card/DeploymentCardLicenseFooter";
import DeploymentCardToolbar from "@features/project-details/components/deployments/deployment-card/DeploymentCardToolbar";
import { useState, type JSX } from "react";
import DeploymentDocumentList from "@deployments/DeploymentDocumentList";
import DeploymentProductList from "@deployments/DeploymentProductList";
import EditDeploymentModal from "@deployments/EditDeploymentModal";
import DeleteDeploymentModal from "@deployments/DeleteDeploymentModal";
import { usePatchDeployment } from "@features/project-details/api/usePatchDeployment";
import { useDownloadDeploymentLicense } from "@features/project-details/api/useDownloadDeploymentLicense";

/**
 * Renders a single deployment environment card with products and documents.
 *
 * @param {DeploymentCardProps} props - Props containing the deployment data.
 * @returns {JSX.Element} The deployment card.
 */
export default function DeploymentCard({
  deployment,
  selectedProduct,
  onToggleProductSelect,
}: DeploymentCardProps): JSX.Element {
  const { name, url, description, createdOn, updatedOn } = deployment;
  const projectId = deployment.project?.id ?? "";
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const patchDeployment = usePatchDeployment();
  const downloadLicense = useDownloadDeploymentLicense();

  const createdAtStr = formatProjectDateTime(createdOn ?? "");
  const updatedAtStr = formatProjectDateTime(updatedOn ?? "");

  const handleDownloadLicense = () => {
    downloadLicense.mutate({
      projectId,
      deploymentId: deployment.id,
      deploymentName: name,
    });
  };

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
          <DeploymentCardToolbar
            onEdit={() => setIsEditModalOpen(true)}
            onDelete={() => setIsDeleteModalOpen(true)}
            isDeleteDisabled={patchDeployment.isPending}
          />
        </Box>

        <Divider />
        <Typography variant="body2" color="text.secondary">
          {displayValue(description, "Not Available")}
        </Typography>
        <Divider />

        <DeploymentProductList
          deploymentId={deployment.id}
          projectId={deployment.project?.id ?? ""}
          selectedProduct={selectedProduct}
          onToggleProductSelect={onToggleProductSelect}
        />

        <Divider />

        <DeploymentDocumentList deploymentId={deployment.id} />

        <Divider />
        <DeploymentCardLicenseFooter
          createdAtLabel={createdAtStr}
          updatedAtLabel={updatedAtStr}
          onDownloadLicense={handleDownloadLicense}
          isDownloading={downloadLicense.isPending}
        />
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
