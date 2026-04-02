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

import { useEffect, useRef, type JSX } from "react";
import { useNavigate, useParams, useLocation } from "react-router";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import useGetCaseDetails from "@api/useGetCaseDetails";
import CaseDetailsContent from "@case-details-details/CaseDetailsContent";

/**
 * ServiceRequestDetailsPage displays a service request with the same shell as
 * case details (tabs, header, actions). Data comes from GET case by id.
 *
 * URL: /:projectId/support|operations/service-requests/:serviceRequestId
 *
 * @returns {JSX.Element} The rendered page.
 */
export default function ServiceRequestDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId, serviceRequestId } = useParams<{
    projectId: string;
    serviceRequestId: string;
  }>();

  const { showLoader, hideLoader } = useLoader();
  const { showError } = useErrorBanner();

  const { data, isLoading, isFetching, isError } = useGetCaseDetails(
    projectId || "",
    serviceRequestId || "",
  );

  const showSkeletons =
    isLoading || isFetching || (data === undefined && !isError);

  useEffect(() => {
    if (showSkeletons) {
      showLoader();
      return () => hideLoader();
    }
    hideLoader();
  }, [showSkeletons, showLoader, hideLoader]);

  const hasShownErrorRef = useRef(false);

  useEffect(() => {
    hasShownErrorRef.current = false;
  }, [serviceRequestId]);

  useEffect(() => {
    if (!isError) {
      hasShownErrorRef.current = false;
      return;
    }
    if (!hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError("Could not load service request details.");
    }
  }, [isError, showError]);

  const handleBack = () => {
    const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;
    if (returnTo) {
      navigate(returnTo);
      return;
    }
    const basePath = location.pathname.includes("/operations/")
      ? "operations"
      : "support";
    navigate(`/projects/${projectId}/${basePath}/service-requests`);
  };

  const handleOpenRelatedCase = () => {
    if (!projectId) return;
    navigate(`/projects/${projectId}/support/chat/create-related-case`, {
      state: {
        relatedCase: {
          parentCaseId: data?.id ?? serviceRequestId ?? "",
          number: data?.number ?? "",
          title: data?.title ?? "",
          description: data?.description ?? "",
          deploymentId: data?.deployment?.id,
          deploymentLabel: data?.deployment?.label,
        },
      },
    });
  };

  return (
    <CaseDetailsContent
      data={data}
      isLoading={showSkeletons}
      isError={isError}
      caseId={serviceRequestId || ""}
      projectId={projectId}
      onBack={handleBack}
      onOpenRelatedCase={handleOpenRelatedCase}
      isServiceRequest
    />
  );
}
