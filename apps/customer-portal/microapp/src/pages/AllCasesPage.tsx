// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { Search } from "@mui/icons-material";
import { InputAdornment, Stack, Tab, Tabs, InputBase as TextField } from "@mui/material";
import { ItemCardExtended, type ItemCardProps } from "@components/features/support";
import { MOCK_EXTENDED_ITEMS } from "@src/mocks/data/support";

export default function AllCasesPage({ type }: { type: ItemCardProps["type"] }) {
  return (
    <Stack gap={2}>
      {MOCK_EXTENDED_ITEMS[type].map((item, index) => (
        <ItemCardExtended key={index} {...item} />
      ))}
    </Stack>
  );
}

export function AllCasesAppBarSlot() {
  return (
    <Stack gap={2} pb={1}>
      <TextField
        fullWidth
        type="search"
        placeholder="Search cases by ID, title, or description..."
        startAdornment={
          <InputAdornment position="start">
            <Search />
          </InputAdornment>
        }
        sx={{
          mt: 1,
          bgcolor: "background.paper",
        }}
      />
      <Tabs
        value={0}
        scrollButtons={false}
        variant="scrollable"
        sx={(theme) => ({
          minHeight: "unset",

          "& .MuiTabs-flexContainer": {
            gap: 1.2,
          },
          "& .MuiButtonBase-root": {
            minHeight: "unset",
            minWidth: "unset",
            p: "4px 12px",
            fontSize: theme.typography.subtitle1,
            color: "text.tertiary",
            fontWeight: "medium",
            textTransform: "revert",
            borderRadius: 999,
            backgroundColor: "background.default",
          },

          "& .MuiTab-root.Mui-selected": {
            color: theme.palette.primary.contrastText,
            backgroundColor: theme.palette.primary.main,
            fontSize: theme.typography.body1,
            fontWeight: "bold",
          },

          "& .MuiTabs-indicator": {
            display: "none",
          },
        })}
      >
        <Tab label="All" disableRipple />
        <Tab label="Open" disableRipple />
        <Tab label="In Progress" disableRipple />
        <Tab label="Waiting" disableRipple />
        <Tab label="Resolved" disableRipple />
        <Tab label="Closed" disableRipple />
      </Tabs>
    </Stack>
  );
}
