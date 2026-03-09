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

import { useNavigate } from "react-router";
import type { JSX } from "react";
import { colors } from "@wso2/oxygen-ui";
import { Server } from "@wso2/oxygen-ui-icons-react";
import RequestCard from "@components/support/request-cards/RequestCard";
import { SERVICE_REQUEST_BULLET_ITEMS } from "@constants/supportConstants";

/**
 * Service Requests card for the Support page.
 * Renders overview and CTAs for service request management.
 *
 * @returns {JSX.Element} The Service Request card.
 */
export default function ServiceRequestCard(): JSX.Element {
  const navigate = useNavigate();

  return (
    <RequestCard
      title="Service Requests"
      subtitle="Manage deployment operations"
      icon={Server}
      paletteKey="info"
      accentColor={colors.blue[600]}
      infoBoxTitle="What are Service Requests?"
      infoBoxDescription="Request operational changes to your managed cloud deployment:"
      bulletItems={SERVICE_REQUEST_BULLET_ITEMS}
      footerButtons={[
        {
          label: "View my requests",
          onClick: () => navigate("service-requests?createdByMe=true"),
        },
        {
          label: "View all requests",
          onClick: () => navigate("service-requests"),
        },
      ]}
      primaryButton={{
        label: "New Service Request",
        onClick: () => navigate("service-requests/create"),
        icon: Server,
      }}
    />
  );
}
