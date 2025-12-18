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

import type { Theme, Components, ChipOwnProps } from "@mui/material";

export default function Chips(theme: Theme): Components {
  return {
    MuiChip: {
      styleOverrides: {
        root: ({ ownerState }: { ownerState?: ChipOwnProps }) => {
          const iconPosition = ownerState?.iconPosition ?? "start";

          return {
            display: "flex",
            gap: 5,

            ...(iconPosition === "end" && {
              flexDirection: "row-reverse",
              "& .MuiChip-icon": {
                gap: 0,
                marginRight: 10,
                marginLeft: 0,
              },
            }),
          };
        },
      },
      variants: [
        {
          props: { color: "success" },
          style: {
            color: theme.palette.semantic.chip.success.text,
            backgroundColor: theme.palette.semantic.chip.success.background,
          },
        },
        {
          props: { color: "warning" },
          style: {
            color: theme.palette.semantic.chip.warning.text,
            backgroundColor: theme.palette.semantic.chip.warning.background,
          },
        },
      ],
    },
  };
}

declare module "@mui/material/Chip" {
  interface ChipOwnProps {
    iconPosition?: "start" | "end";
  }
}
