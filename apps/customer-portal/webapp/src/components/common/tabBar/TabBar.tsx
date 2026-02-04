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

import React, { type JSX } from "react";
import { Box, Button, Card } from "@wso2/oxygen-ui";

export interface TabOption {
  id: string;
  label: string;
  icon?: React.ElementType;
  count?: number | string;
  badgeColor?: string;
}

export interface TabBarProps {
  tabs: TabOption[];
  activeTab: string;
  onTabChange: (tabId: string) => void;
  className?: string;
}

const TabBar = ({ tabs, activeTab, onTabChange }: TabBarProps): JSX.Element => {
  return (
    <Card
      role="tablist"
      sx={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        p: 0.5,
        mb: 3,
        height: 36,
        width: "fit-content",
      }}
    >
      {tabs.map((tab) => {
        const isActive = activeTab === tab.id;
        const Icon = tab.icon;

        return (
          <Button
            key={tab.id}
            role="tab"
            variant="outlined"
            aria-selected={isActive}
            onClick={() => onTabChange(tab.id)}
            startIcon={Icon ? <Icon size={16} /> : undefined}
            sx={{
              position: "relative",
              flex: 1,
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              whiteSpace: "nowrap",
              border: "1px solid",
              px: 2,
              py: 0.5,
              textTransform: "none",
              color: isActive ? "warning.main" : "text.secondary",
              borderColor: isActive ? "warning.main" : "transparent",
              transition: "all 0.2s ease-in-out",
              minWidth: "auto",
              height: "100%",
              "&:hover": {
                bgcolor: isActive ? "background.paper" : "text.secondary.light",
              },
            }}
          >
            {tab.label}
            {tab.count !== undefined && (
              <Box
                component="span"
                sx={{
                  ml: 1,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  bgcolor: tab.badgeColor || "warning.main",
                  color: "white",
                  borderRadius: "50%",
                  width: 16,
                  height: 16,
                  fontSize: "0.625rem",
                  lineHeight: "16px",
                }}
              >
                {tab.count}
              </Box>
            )}
          </Button>
        );
      })}
    </Card>
  );
};

export default TabBar;
