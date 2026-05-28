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
import { useNavigate } from "react-router-dom";

import { useProject } from "@context/project";

import type { Case, CaseClassificationResponseDto } from "@features/case-types/cases/types";
import type { BubbleProps } from "@features/case-types/conversations/components";

import { CASE_TYPES, OUTSTANDING_CASES_BY_SEVERITY_TITLE, ROUTES } from "@shared/constants";

export const useCaseNavigation = () => {
  const navigate = useNavigate();
  const { noveraEnabled } = useProject();

  return {
    toBySeverity: (id: string | number, label: string) =>
      navigate(
        {
          pathname: "/support/all",
          search: new URLSearchParams([
            ["type", CASE_TYPES.DEFAULT],
            ["severity", String(id)],
          ]).toString(),
        },
        { state: { title: OUTSTANDING_CASES_BY_SEVERITY_TITLE(label) } },
      ),

    toCaseCreate: () => navigate(noveraEnabled ? ROUTES[CASE_TYPES.CHAT].create : ROUTES[CASE_TYPES.DEFAULT].create),
    toRelativeCaseCreate: (data: Case) => navigate(ROUTES[CASE_TYPES.DEFAULT].create, { state: { case: data } }),
    toClassifiedCaseCreate: (messages: BubbleProps[], classifications: CaseClassificationResponseDto) =>
      navigate(ROUTES[CASE_TYPES.DEFAULT].create, { state: { messages, classifications } }),
  };
};
