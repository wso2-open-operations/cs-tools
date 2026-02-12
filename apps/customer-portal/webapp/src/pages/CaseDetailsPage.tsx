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
import useGetCaseDetails from "@api/useGetCaseDetails";
import CaseDetailsContent from "@case-details/CaseDetailsContent";

/**
 * CaseDetailsPage displays details for a single support case.
 *
 * @returns {JSX.Element} The rendered Case Details page.
 */
export default function CaseDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId, caseId } = useParams<{ projectId: string; caseId: string }>();
  const { showLoader, hideLoader } = useLoader();
  const { showError } = useErrorBanner();

  const {
    data,
    isLoading,
    isFetching,
    isError,
  } = useGetCaseDetails(projectId || "", caseId || "");

  // Show skeletons immediately when no data (avoid "-" flash on refresh) and when loading/refetching.
  const showSkeletons =
    isLoading ||
    isFetching ||
    (data === undefined && !isError);

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
    if (!hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError("Could not load case details.");
    }
  }, [isError, showError]);

  const handleBack = () => {
    navigate(`/${projectId}/support/cases`);
  };

  return (
    <CaseDetailsContent
      data={data}
      isLoading={showSkeletons}
      isError={isError}
      caseId={caseId || ""}
      onBack={handleBack}
    />
  );
}
