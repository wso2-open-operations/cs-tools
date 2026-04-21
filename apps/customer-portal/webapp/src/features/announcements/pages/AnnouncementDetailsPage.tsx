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
import { useNavigate, useParams } from "react-router";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import useGetCaseDetails from "@features/support/api/useGetCaseDetails";
import AnnouncementDetailsPanel from "@features/announcements/components/AnnouncementDetailsPanel";
import { ANNOUNCEMENT_DETAILS_FETCH_ERROR_BANNER } from "@features/announcements/constants/announcementsConstants";

/**
 * AnnouncementDetailsPage displays details for a single announcement (case).
 * Fetches case details via useGetCaseDetails and renders the announcement-specific layout.
 *
 * @returns {JSX.Element} The rendered Announcement Details page.
 */
export default function AnnouncementDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId, caseId } = useParams<{
    projectId: string;
    caseId: string;
  }>();
  const { showLoader, hideLoader } = useLoader();
  const { showError } = useErrorBanner();

  const { data, isLoading, isError } = useGetCaseDetails(
    projectId || "",
    caseId || "",
  );

  const showSkeletons =
    isLoading || (data === undefined && !isError);

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
  }, [caseId]);

  useEffect(() => {
    if (!isError) {
      hasShownErrorRef.current = false;
      return;
    }
    if (!hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError(ANNOUNCEMENT_DETAILS_FETCH_ERROR_BANNER);
    }
  }, [isError, showError]);

  const handleBack = () => {
    navigate(`/projects/${projectId}/announcements`, { state: { fromBack: true } });
  };

  return (
    <AnnouncementDetailsPanel
      data={data}
      isLoading={showSkeletons}
      isError={isError}
      caseId={caseId || ""}
      projectId={projectId || ""}
      onBack={handleBack}
    />
  );
}
