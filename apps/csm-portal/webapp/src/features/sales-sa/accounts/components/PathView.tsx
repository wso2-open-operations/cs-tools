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

// "Home" navigates to /spl/cases, not this app's own /cases — that route is
// CSM Portal's own, unrelated case-management feature.
import { Box, Button, Tooltip } from "@wso2/oxygen-ui";
import { ChevronRightIcon, HomeIcon } from "@wso2/oxygen-ui-icons-react";
import { useNavigate } from "react-router";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";

export default function PathView({ accountName, accountNumber }: { accountName?: string; accountNumber?: string }) {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();

  const handleAccountClick = () => {
    if (accountNumber) {
      navigate(`/spl/accounts/${accountNumber}`);
    } else {
      showError("Account not found.");
    }
  };

  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, mb: 1.5 }}>
      <Tooltip title="Home">
        <Button size="small" onClick={() => navigate("/spl/cases")} startIcon={<HomeIcon size={16} />} />
      </Tooltip>
      <ChevronRightIcon size={16} style={{ opacity: 0.5 }} />
      <Tooltip title="Account">
        <Button size="small" onClick={handleAccountClick} sx={{ textTransform: "none", fontWeight: 600 }}>
          {accountName}
        </Button>
      </Tooltip>
    </Box>
  );
}
