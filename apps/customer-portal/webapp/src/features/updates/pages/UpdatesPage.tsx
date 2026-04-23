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

import { Box } from "@wso2/oxygen-ui";
import { useParams } from "react-router";
import { useState, useEffect, useRef, type JSX } from "react";
import TabBar from "@components/tab-bar/TabBar";
import { UpdatesStatsGrid } from "@features/updates/components/stat-card-row/UpdatesStatsGrid";
import { UpdateProductGrid } from "@update-cards/UpdateProductGrid";
import AllUpdatesTab from "@features/updates/components/all-updates/AllUpdatesTab";
import { useGetRecommendedUpdateLevels } from "@features/updates/api/useGetRecommendedUpdateLevels";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useLogger } from "@hooks/useLogger";
import {
  UPDATES_PAGE_TABS,
  UPDATES_RECOMMENDED_LEVELS_LOAD_ERROR,
} from "@features/updates/constants/updatesConstants";
import { UpdatesPageTabId } from "@features/updates/types/updates";

/**
 * UpdatesPage component to display project updates with tab bar and stats.
 *
 * @returns {JSX.Element} The rendered Updates page.
 */
export default function UpdatesPage(): JSX.Element {
  const logger = useLogger();
  const { projectId } = useParams<{ projectId: string }>();
  const [activeTab, setActiveTab] = useState<string>(UpdatesPageTabId.MyUpdates);
  const { showLoader, hideLoader } = useLoader();
  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);

  const { data, isLoading, isError } = useGetRecommendedUpdateLevels();

  const isUpdatesLoading = isLoading || (!data && !isError);

  useEffect(() => {
    if (isUpdatesLoading) {
      showLoader();
      return () => hideLoader();
    }
  }, [isUpdatesLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (data) {
      logger.debug(`Updates data loaded for project ID: ${projectId}`);
    }
  }, [data, logger, projectId]);

  useEffect(() => {
    if (isError && !hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError(UPDATES_RECOMMENDED_LEVELS_LOAD_ERROR);
      logger.error(
        `Failed to load recommended update levels for project ID: ${projectId}`,
      );
    }
    if (!isError) {
      hasShownErrorRef.current = false;
    }
  }, [isError, showError, logger, projectId]);

  return (
    <Box sx={{ width: "100%", pt: 0 }}>
      <UpdatesStatsGrid data={data} isLoading={isLoading} isError={isError} />
      <Box sx={{ mt: 2 }}>
      <TabBar
        tabs={UPDATES_PAGE_TABS}
        activeTab={activeTab}
        onTabChange={setActiveTab}
      />
      </Box>
      {activeTab === UpdatesPageTabId.MyUpdates ? (
        <UpdateProductGrid
          data={data}
          isLoading={isLoading}
          isError={isError}
          projectId={projectId}
        />
      ) : (
        <AllUpdatesTab />
      )}
    </Box>
  );
}
