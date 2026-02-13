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

import { CASE_DETAILS_TABS } from "@constants/supportConstants";
import { Box, Button, Tab, Tabs } from "@wso2/oxygen-ui";
import { Maximize2, Minimize2 } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

export interface CaseDetailsTabsProps {
  value: number;
  onChange: (_e: unknown, newValue: number) => void;
  focusMode?: boolean;
  onFocusModeToggle?: () => void;
  attachmentCount?: number;
}

/**
 * Tab bar for case details: Activity, Details, Attachments, Calls, Knowledge Base.
 *
 * @param {CaseDetailsTabsProps} props - Controlled value and onChange.
 * @returns {JSX.Element} The tabs wrapper.
 */
export default function CaseDetailsTabs({
  value,
  onChange,
  focusMode = false,
  onFocusModeToggle,
  attachmentCount,
}: CaseDetailsTabsProps): JSX.Element {
  return (
    <Box
      sx={{
        borderBottom: 1,
        borderColor: "divider",
        mt: focusMode ? 0 : 2,
        mx: -2,
        mb: -2,
        display: "flex",
        alignItems: "center",
      }}
    >
      <Tabs
        value={value}
        onChange={onChange}
        sx={{
          flex: 1,
          px: 2,
          minHeight: 40,
          "& .MuiTab-root": {
            minHeight: 40,
            py: 1.5,
            textTransform: "none",
            fontSize: "0.75rem",
          },
          "& .MuiTabs-indicator": { bgcolor: "primary.main" },
        }}
      >
        {CASE_DETAILS_TABS.map(({ label, Icon }) => {
          const isAttachmentsTab = label.startsWith("Attachments");
          const tabLabel =
            isAttachmentsTab && attachmentCount !== undefined
              ? `Attachments (${attachmentCount})`
              : label;
          return (
            <Tab
              key={label}
              icon={<Icon size={14} />}
              iconPosition="start"
              label={tabLabel}
            />
          );
        })}
      </Tabs>
      {onFocusModeToggle && (
        <Button
          aria-label={focusMode ? "Exit focus mode" : "Focus mode"}
          startIcon={
            focusMode ? <Minimize2 size={14} /> : <Maximize2 size={14} />
          }
          onClick={onFocusModeToggle}
          size="small"
          sx={{ mr: 2, flexShrink: 0 }}
          variant="text"
        >
          {focusMode ? "Exit Focus Mode" : "Focus Mode"}
        </Button>
      )}
    </Box>
  );
}
