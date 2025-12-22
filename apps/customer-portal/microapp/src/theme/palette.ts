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

export const palette = {
  primary: {
    main: "#ff7300",
    contrastText: "#ffffff",
  },
  background: {
    default: "#f5f5f5",
    paper: "#ffffff",
    card: "#f5f5f599",
  },
  text: {
    tertiary: "#b8b8b8",
  },
  semantic: {
    border: {
      subtle: "#e0e0e0",
    },
    state: {
      active: {
        background: "#fef8f3",
      },
    },
    portal: {
      background: { main: "#f5f5f5", secondary: "#f9fafb" },
      accent: {
        orange: "#ff5722",
        yellow: "#ffc107",
        green: "#4caf50",
        blue: "#5b6ef5",
        cyan: "#00bcd4",
        purple: "#9c27b0",
        amber: "#ffc107",
      },
    },
    chip: {
      success: {
        text: "#4daf50",
        background: "#e8f5e9",
      },
      warning: {
        text: "#ffc107",
        background: "#fff8e1",
      },
      error: {
        text: "#f54a00",
        background: "#ffedd5",
      },
    },
    priority: {
      low: {
        background: "#f3f4f6",
        text: "#374151",
      },
      normal: {
        background: "#dbeafe",
        text: "#1d4ed8",
      },
      high: {
        background: "#fee2e2",
        text: "#b91c1c",
      },
    },
    status: {
      wip: "#f97316",
      waiting: "#3b82f6",
      awaiting: "#a855f7",
    },
    avatar: {
      foreground: "#ca3500",
      background: "#ffedd4",
    },
  },
  divider: "#eeeeee",
} as const;

declare module "@mui/material/styles" {
  interface Palette {
    semantic: typeof palette.semantic;
  }
}
