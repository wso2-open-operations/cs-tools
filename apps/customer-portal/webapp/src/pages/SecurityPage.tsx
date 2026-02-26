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

import { useState, useCallback, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import { Box } from "@wso2/oxygen-ui";
import { Siren, Package } from "@wso2/oxygen-ui-icons-react";
//import SecurityStats from "@components/security/SecurityStats";
import TabBar from "@components/common/tab-bar/TabBar";
import ProductVulnerabilitiesTable from "@components/security/ProductVulnerabilitiesTable";
import ComponentAnalysis from "@components/security/ComponentAnalysis";

const SecurityPage = (): JSX.Element => {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const [activeTab, setActiveTab] = useState("components");

  const handleVulnerabilityClick = useCallback(
    (vulnerability: { id: string }) => {
      navigate(`/${projectId}/security-center/${vulnerability.id}`);
    },
    [navigate, projectId],
  );

  const tabs = [
    {
      id: "components",
      label: "Component Analysis",
      icon: Package,
    },
    {
      id: "vulnerabilities",
      label: "Security Report Analysis",
      icon: Siren,
    },
  ];

  return (
    <Box>
      {/* <SecurityStats /> */}

      <Box>
        <TabBar tabs={tabs} activeTab={activeTab} onTabChange={setActiveTab} />

        <Box>
          {activeTab === "components" && (
            <ProductVulnerabilitiesTable
              onVulnerabilityClick={handleVulnerabilityClick}
            />
          )}
          {activeTab === "vulnerabilities" && <ComponentAnalysis />}
        </Box>
      </Box>
    </Box>
  );
};

export default SecurityPage;
