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
import { CalendarDays } from "@wso2/oxygen-ui-icons-react";
import RequestCard from "@components/support/request-cards/RequestCard";
import { CHANGE_REQUEST_BULLET_ITEMS } from "@constants/supportConstants";

/**
 * Change Requests card for the Support page.
 * Renders overview and CTA for change request management.
 *
 * @returns {JSX.Element} The Change Request card.
 */
export default function ChangeRequestCard(): JSX.Element {
  const navigate = useNavigate();

  return (
    <RequestCard
      title="Change Requests"
      subtitle="Track infrastructure changes"
      icon={CalendarDays}
      paletteKey="secondary"
      accentColor={colors.purple[600]}
      infoBoxTitle="What are Change Requests?"
      infoBoxDescription="Structured workflow for planned infrastructure changes:"
      bulletItems={CHANGE_REQUEST_BULLET_ITEMS}
      secondaryButtonLabel="View All Change Requests"
      onSecondaryClick={() => navigate("../dashboard")}
    />
  );
}
