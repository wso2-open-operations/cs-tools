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
import { useLocation, useNavigate, useParams } from "react-router";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import useGetCaseDetails from "@features/support/api/useGetCaseDetails";
import CaseDetailsContent from "@case-details-details/CaseDetailsContent";
import {
  isAnnouncementType,
  isEngagementType,
  isSecurityReportAnalysisType,
  isServiceRequestType,
} from "@features/support/utils/support";
import { SecurityTabId } from "@features/security/types/security";
import { ROUTE_PREVIOUS_PAGE } from "@features/project-hub/constants/navigationConstants";

/**
 * CaseDetailsPage displays details for a single support case.
 *
 * @returns {JSX.Element} The rendered Case Details page.
 */
export default function CaseDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId, caseId } = useParams<{
    projectId: string;
    caseId: string;
  }>();
  const { showLoader, hideLoader } = useLoader();
  const { showError } = useErrorBanner();

  const { data, isLoading, isError, error } = useGetCaseDetails(
    projectId || "",
    caseId || "",
  );

  const isEngagementRoute = location.pathname.includes("/engagements/");
  const isSecurityReportAnalysisRoute = location.pathname.includes(
    "security-report-analysis",
  );

  // Announcements and Service Requests have their own dedicated pages; Engagement and
  // Security Report Analysis cases have dedicated routes too, though they share this same
  // component. Redirect when a case is loaded on a route that doesn't match its actual type.
  const isAnnouncement = isAnnouncementType(data?.type);
  const isMisroutedEngagement = isEngagementType(data?.type) && !isEngagementRoute;
  const isMisroutedSecurityReportAnalysis =
    isSecurityReportAnalysisType(data?.type) && !isSecurityReportAnalysisRoute;
  const isServiceRequest = isServiceRequestType(data?.type);
  const isMisrouted =
    isAnnouncement ||
    isMisroutedEngagement ||
    isMisroutedSecurityReportAnalysis ||
    isServiceRequest;

  useEffect(() => {
    if (!projectId || !caseId) return;
    if (isAnnouncement) {
      navigate(`/projects/${projectId}/announcements/${caseId}`, {
        replace: true,
      });
    } else if (isMisroutedEngagement) {
      navigate(`/projects/${projectId}/engagements/${caseId}`, {
        replace: true,
      });
    } else if (isMisroutedSecurityReportAnalysis) {
      navigate(
        `/projects/${projectId}/security-center/security-report-analysis/${caseId}`,
        { replace: true },
      );
    } else if (isServiceRequest) {
      navigate(`/projects/${projectId}/operations/service-requests/${caseId}`, {
        replace: true,
      });
    }
  }, [
    isAnnouncement,
    isMisroutedEngagement,
    isMisroutedSecurityReportAnalysis,
    isServiceRequest,
    projectId,
    caseId,
    navigate,
  ]);

  // Show skeletons immediately when no data (avoid "-" flash on refresh) and when loading/refetching.
  const showSkeletons =
    isLoading || (data === undefined && !isError) || isMisrouted;

  useEffect(() => {
    if (showSkeletons) {
      showLoader();
      return () => hideLoader();
    }
    hideLoader();
  }, [showSkeletons, showLoader, hideLoader]);

  const hasShownErrorRef = useRef(false);

  // Reset so error can show again when navigating to another case or when error clears.
  useEffect(() => {
    hasShownErrorRef.current = false;
  }, [caseId]);

  useEffect(() => {
    if (!isError) {
      hasShownErrorRef.current = false;
      return;
    }
    if (error) {
      // In-page ApiErrorState handles this path; avoid duplicate banner noise.
      return;
    }
    if (!hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError("Could not load case details.");
    }
  }, [isError, error, showError]);

  const handleBack = () => {
    const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;
    if (returnTo) {
      navigate(returnTo, { state: { fromBack: true } });
      return;
    }
    if (isEngagementRoute) {
      navigate(`/projects/${projectId}/engagements`, { state: { fromBack: true } });
      return;
    }

    const queryTab = new URLSearchParams(location.search).get("tab");
    const caseDetailsWithFlag = data as
      | ({ isSecurityReport?: boolean } & typeof data)
      | undefined;

    const isSecurityReport =
      caseDetailsWithFlag?.isSecurityReport === true ||
      isSecurityReportAnalysisType(data?.type) ||
      isSecurityReportAnalysisRoute ||
      queryTab === SecurityTabId.VULNERABILITIES;

    if (isSecurityReport) {
      navigate(
        `/projects/${projectId}/security-center?tab=${SecurityTabId.VULNERABILITIES}`,
        { state: { fromBack: true } },
      );
    } else {
      navigate(ROUTE_PREVIOUS_PAGE);
    }
  };

  const handleOpenRelatedCase = () => {
    if (!projectId) return;
    const deployedProductLabel = [
      data?.deployedProduct?.label ?? "",
      data?.deployedProduct?.version ?? "",
    ]
      .join(" ")
      .trim();
    navigate(`/projects/${projectId}/support/chat/create-related-case`, {
      state: {
        relatedCase: {
          relatedCaseId: data?.id ?? caseId ?? "",
          number: data?.number ?? "",
          title: data?.title ?? "",
          description: data?.description ?? "",
          deploymentId: data?.deployment?.id,
          deploymentLabel: data?.deployment?.label,
          deployedProductId: data?.deployedProduct?.id,
          deployedProductLabel:
            deployedProductLabel || data?.deployedProduct?.label,
        },
      },
    });
  };

  return (
    <CaseDetailsContent
      data={data}
      isLoading={showSkeletons}
      isError={isError}
      error={error}
      caseId={caseId || ""}
      projectId={projectId}
      onBack={handleBack}
      onOpenRelatedCase={handleOpenRelatedCase}
      hideActionRow={false}
    />
  );
}
