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

import type { ActivityTimelineEntryProps } from "@components/features/detail";

export const MOCK_ACTIVITY_TIMELINE: Omit<ActivityTimelineEntryProps, "variant">[] = [
  {
    author: "System",
    title: "created this case",
    timestamp: "2 days ago",
  },
  {
    author: "System",
    title: "assigned to Support Team",
    timestamp: "2 days ago",
  },
  {
    author: "System",
    title: "assigned to Support Team",
    timestamp: "2 days ago",
  },
  {
    author: "System",
    timestamp: "2 days ago",
    comment: "I've reviewed your case. Could you please share the error logs from your authentication service?",
    attachment: "auth-service-logs.txt",
  },
];
