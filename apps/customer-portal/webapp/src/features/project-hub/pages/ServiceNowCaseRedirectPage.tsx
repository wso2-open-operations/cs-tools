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

import { useEffect, type JSX } from "react";
import { useNavigate, useSearchParams } from "react-router";
import useGetCaseDetails from "@features/support/api/useGetCaseDetails";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import {
  SERVICENOW_REDIRECT_NO_CASE_ID,
  SERVICENOW_REDIRECT_RESOLVE_ERROR,
} from "@features/project-hub/constants/projectHubConstants";

/**
 * Handles ServiceNow deep-links of the form:
 *   /support?id=csm_ticket&table=sn_customerservice_case&sys_id=<caseId>&view=csp&spa=1
 *
 * Resolves the project from the case details and redirects to:
 *   /projects/<projectId>/support/cases/<caseId>
 *
 * Falls back to the project hub if the case or project cannot be resolved.
 *
 * @returns {JSX.Element} An empty element — this page only performs a redirect.
 */
export default function ServiceNowCaseRedirectPage(): JSX.Element {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { showLoader, hideLoader } = useLoader();
  const { showError } = useErrorBanner();

  const caseId = searchParams.get("sys_id") ?? "";

  const { data, isLoading, isError } = useGetCaseDetails(undefined, caseId);

  useEffect(() => {
    if (!caseId) {
      showError(SERVICENOW_REDIRECT_NO_CASE_ID);
      void navigate("/404", { replace: true });
      return;
    }
    if (isLoading) {
      showLoader();
      return () => hideLoader();
    }
  }, [isLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (isError || (!isLoading && data && !data.project?.id)) {
      showError(SERVICENOW_REDIRECT_RESOLVE_ERROR);
      void navigate("/404", { replace: true });
      return;
    }

    if (data?.project?.id) {
      void navigate(`/projects/${data.project.id}/support/cases/${caseId}`, {
        replace: true,
      });
    }
  }, [data, isError, isLoading, caseId, navigate, showError]);

  return <></>;
}
