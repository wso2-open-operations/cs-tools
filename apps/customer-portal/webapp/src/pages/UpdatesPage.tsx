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

import { Box, Typography } from "@wso2/oxygen-ui";
import { useParams } from "react-router";
import { useState, useEffect, useRef, type JSX } from "react";
import TabBar from "@components/common/tab-bar/TabBar";
import { UpdatesStatsGrid } from "@components/updates/UpdatesStatsGrid";
import { useGetProductUpdatesStats } from "@api/useGetProductUpdatesStats";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useLogger } from "@hooks/useLogger";

const UPDATES_TABS = [
  { id: "my-updates", label: "My Updates" },
  { id: "all", label: "All Updates" },
];

/**
 * UpdatesPage component to display project updates with tab bar and stats.
 *
 * @returns {JSX.Element} The rendered Updates page.
 */
export default function UpdatesPage(): JSX.Element {
  const logger = useLogger();
  const { projectId } = useParams<{ projectId: string }>();
  const [activeTab, setActiveTab] = useState("my-updates");
  const { showLoader, hideLoader } = useLoader();
  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);

  const { data, isLoading, isError } = useGetProductUpdatesStats(
    projectId || "",
  );

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
      showError("updates statistics");
      logger.error(`Failed to load updates stats for project ID: ${projectId}`);
    }
    if (!isError) {
      hasShownErrorRef.current = false;
    }
  }, [isError, showError, logger, projectId]);

  const renderContent = (): JSX.Element => {
    if (activeTab === "my-updates") {
      return (
        <UpdatesStatsGrid data={data} isLoading={isLoading} isError={isError} />
      );
    }
    return (
      <Typography variant="body1" color="text.secondary">
        All updates content
      </Typography>
    );
  };

  return (
    <Box sx={{ width: "100%", pt: 0 }}>
      <TabBar
        tabs={UPDATES_TABS}
        activeTab={activeTab}
        onTabChange={setActiveTab}
      />
      {renderContent()}
    </Box>
  );
}
