// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import EmptyState from "@components/empty-state/EmptyState";
import error500Svg from "@assets/error/error-500.svg";
import { getApiErrorMessage } from "@utils/ApiError";
import ErrorBanner from "@components/error-banner/ErrorBanner";
import SuccessBanner from "@components/success-banner/SuccessBanner";
import AddDeploymentModal from "@features/project-details/components/deployments/AddDeploymentModal";
import DeploymentCard from "@features/project-details/components/deployments/DeploymentCard";
import DeploymentCardSkeleton from "@features/project-details/components/deployments/DeploymentCardSkeleton";
import DeploymentHeader from "@features/project-details/components/deployments/DeploymentHeader";
import {
  Box,
  Button,
  Grid,
  Pagination,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import type { SelectedDeploymentProduct } from "@features/project-details/types/deployments";
import type { ProjectDeploymentsProps } from "@features/project-details/types/projectDetailsComponents";
import {
  buildServiceRequestCreateSearchParams,
  clampDeploymentsPage,
} from "@features/project-details/utils/deployments";
import { Plus, Server } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useEffect, useState, type JSX } from "react";
import { useNavigate } from "react-router";

/**
 * Displays deployment environments for a project.
 *
 * @param {ProjectDeploymentsProps} props - Props including projectId.
 * @returns {JSX.Element} The project deployments section.
 */
export default function ProjectDeployments({
  projectId,
}: ProjectDeploymentsProps): JSX.Element {
  const navigate = useNavigate();
  const PAGE_SIZE = 10;
  const [page, setPage] = useState(1);

  const deploymentsQuery = usePostProjectDeploymentsSearchInfinite(projectId, {
    pageSize: PAGE_SIZE,
    enabled: !!projectId,
  });
  const showLoading = deploymentsQuery.isLoading || deploymentsQuery.isPending;
  const isError = deploymentsQuery.isError;

  const totalRecords =
    deploymentsQuery.data?.pages?.[0]?.totalRecords ?? undefined;
  const totalPages =
    totalRecords != null ? Math.ceil(totalRecords / PAGE_SIZE) : 0;

  const clampedPage = clampDeploymentsPage(page, totalPages);
  const currentPageIndex = Math.max(0, clampedPage - 1);
  const currentDeployments =
    deploymentsQuery.data?.pages?.[currentPageIndex]?.deployments ?? [];

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [selectedProduct, setSelectedProduct] =
    useState<SelectedDeploymentProduct | null>(null);

  const isPageLoaded =
    (deploymentsQuery.data?.pages?.length ?? 0) > currentPageIndex;
  const isShowingDeployments = currentDeployments.length > 0;

  useEffect(() => {
    if (!projectId) return;
    if (!clampedPage) return;
    if (isPageLoaded) return;
    if (!deploymentsQuery.hasNextPage) return;
    if (deploymentsQuery.isFetchingNextPage) return;
    void deploymentsQuery.fetchNextPage();
  }, [projectId, clampedPage, isPageLoaded, deploymentsQuery]);

  const handleOpenModal = () => setIsModalOpen(true);
  const handleCloseModal = () => setIsModalOpen(false);
  const handleSuccess = useCallback(
    () => setSuccessMessage("Deployment created successfully."),
    [],
  );
  const handleError = useCallback(
    (message: string) => setErrorMessage(message),
    [],
  );

  const handleToggleProductSelect = useCallback(
    (deploymentId: string, productItemId: string) => {
      setSelectedProduct((prev) => {
        if (
          prev?.deploymentId === deploymentId &&
          prev.productItemId === productItemId
        ) {
          return null;
        }
        return { deploymentId, productItemId };
      });
    },
    [],
  );

  const handleCreateServiceRequest = useCallback(() => {
    if (!selectedProduct || !projectId) return;
    const q = buildServiceRequestCreateSearchParams(selectedProduct);
    navigate(
      `/projects/${projectId}/support/service-requests/create?${q.toString()}`,
    );
  }, [navigate, projectId, selectedProduct]);

  if (!projectId) {
    return (
      <Box sx={{ p: 3, textAlign: "center" }}>
        <Typography color="error">Invalid Project ID.</Typography>
      </Box>
    );
  }

  const banners = (
    <>
      {successMessage && (
        <SuccessBanner
          message={successMessage}
          onClose={() => setSuccessMessage(null)}
        />
      )}
      {errorMessage && (
        <ErrorBanner
          message={errorMessage}
          onClose={() => setErrorMessage(null)}
        />
      )}
    </>
  );

  const deploymentsToolbar = (showing: number, loading: boolean) => (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        mb: 2,
        flexWrap: "wrap",
        gap: 1,
      }}
    >
      {loading ? (
        <Skeleton variant="text" width={200} height={24} />
      ) : (
        <Typography variant="body2" color="text.secondary">
          {totalRecords != null ? (
            <>
              Showing {Math.min(clampedPage * PAGE_SIZE, totalRecords)} of{" "}
              {totalRecords} deployment environment
              {totalRecords !== 1 ? "s" : ""}
            </>
          ) : (
            <>
              {showing} deployment environment{showing !== 1 ? "s" : ""}
            </>
          )}
          {selectedProduct ? (
            <Typography component="span" sx={{ color: "primary.main", ml: 1 }}>
              · 1 product selected
            </Typography>
          ) : null}
        </Typography>
      )}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        {loading ? (
          <>
            <Skeleton variant="rounded" width={200} height={32} />
            <Skeleton variant="rounded" width={140} height={32} />
          </>
        ) : (
          <>
            <Button
              variant="contained"
              color="warning"
              size="small"
              disabled={!selectedProduct}
              startIcon={<Server size={16} aria-hidden />}
              onClick={handleCreateServiceRequest}
            >
              {`Create Service Request${selectedProduct ? " (1)" : ""}`}
            </Button>
            <Button
              variant="contained"
              color="warning"
              size="small"
              startIcon={<Plus size={16} aria-hidden />}
              onClick={handleOpenModal}
            >
              Add Deployment
            </Button>
          </>
        )}
      </Box>
    </Box>
  );

  const renderContent = () => {
    if (showLoading) {
      return (
        <>
          {deploymentsToolbar(0, true)}
          <Grid container spacing={3}>
            {[1, 2, 3].map((i) => (
              <Grid key={i} size={12}>
                <DeploymentCardSkeleton />
              </Grid>
            ))}
          </Grid>
        </>
      );
    }

    if (isError) {
      const backendMessage = getApiErrorMessage(deploymentsQuery.error);
      return (
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            flexDirection: "column",
            p: 5,
          }}
        >
          <img src={error500Svg} alt="" aria-hidden="true" style={{ width: 200, height: "auto" }} />
          <Typography
            role="alert"
            aria-live="assertive"
            variant="body2"
            color="text.secondary"
            sx={{ mt: 2, textAlign: "center" }}
          >
            {backendMessage ?? "Failed to load deployments. Please try again."}
          </Typography>
        </Box>
      );
    }

    if (!isShowingDeployments && (totalRecords ?? 0) === 0) {
      return (
        <>
          <DeploymentHeader count={0} onAddClick={handleOpenModal} />
          <EmptyState description="It seems there are no deployments associated with this project." />
        </>
      );
    }

    return (
      <>
        {deploymentsToolbar(currentDeployments.length, false)}
        <Grid container spacing={3}>
          {currentDeployments.map((deployment) => (
            <Grid key={deployment.id} size={12}>
              <DeploymentCard
                deployment={deployment}
                selectedProduct={selectedProduct}
                onToggleProductSelect={handleToggleProductSelect}
              />
            </Grid>
          ))}
        </Grid>

        {totalPages > 1 && (
          <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 4 }}>
            <Pagination
              count={totalPages}
              page={clampedPage}
              onChange={(_, value) => setPage(value)}
              color="primary"
              variant="outlined"
              shape="rounded"
            />
          </Box>
        )}
      </>
    );
  };

  return (
    <Box>
      {banners}
      {renderContent()}
      <AddDeploymentModal
        open={isModalOpen}
        projectId={projectId}
        onClose={handleCloseModal}
        onSuccess={handleSuccess}
        onError={handleError}
      />
    </Box>
  );
}
