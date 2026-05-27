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
import { Tabs as MuiTabs, Tab } from "@wso2/oxygen-ui";

import { useProject } from "@context/project";

import { CASE_TYPE_PLURAL_LABELS, CASE_TYPES } from "@shared/constants";
import type { CaseType } from "@shared/types";

interface TabsProps {
  value: CaseType;
  onTabChange: (value: CaseType) => void;
}

export function Tabs({ value, onTabChange }: TabsProps) {
  const {
    features: {
      hasServiceRequestReadAccess,
      hasChangeRequestReadAccess,
      hasEngagementsReadAccess,
      hasSraReadAccess,
    } = {},
  } = useProject();

  return (
    <MuiTabs variant="scrollable" value={value} onChange={(_, value) => onTabChange(value)}>
      <Tab label={CASE_TYPE_PLURAL_LABELS[CASE_TYPES.DEFAULT]} value={CASE_TYPES.DEFAULT} disableRipple />

      <Tab label={CASE_TYPE_PLURAL_LABELS[CASE_TYPES.CHAT]} value={CASE_TYPES.CHAT} disableRipple />

      {hasServiceRequestReadAccess && (
        <Tab
          label={CASE_TYPE_PLURAL_LABELS[CASE_TYPES.SERVICE_REQUEST]}
          value={CASE_TYPES.SERVICE_REQUEST}
          disableRipple
        />
      )}

      {hasChangeRequestReadAccess && (
        <Tab
          label={CASE_TYPE_PLURAL_LABELS[CASE_TYPES.CHANGE_REQUEST]}
          value={CASE_TYPES.CHANGE_REQUEST}
          disableRipple
        />
      )}

      {hasSraReadAccess && (
        <Tab
          label={CASE_TYPE_PLURAL_LABELS[CASE_TYPES.SECURITY_REPORT_ANALYSIS]}
          value={CASE_TYPES.SECURITY_REPORT_ANALYSIS}
          disableRipple
        />
      )}

      {hasEngagementsReadAccess && (
        <Tab label={CASE_TYPE_PLURAL_LABELS[CASE_TYPES.ENGAGEMENT]} value={CASE_TYPES.ENGAGEMENT} disableRipple />
      )}

      <Tab label={CASE_TYPE_PLURAL_LABELS[CASE_TYPES.ANNOUNCEMENT]} value={CASE_TYPES.ANNOUNCEMENT} disableRipple />
    </MuiTabs>
  );
}
