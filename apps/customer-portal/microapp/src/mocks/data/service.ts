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

import type { ProgressTimelineEntryProps } from "@components/features/detail";

export const MOCK_REQUIREMENTS = [
  "New staging environment for API Manager",
  "Same configuration as production",
  "Access for development team (5 users)",
  "Integration with existing LDAP",
  "SSL certificates configured",
];

export const MOCK_TIMELINE_DATA: Omit<ProgressTimelineEntryProps, "variant">[] = [
  {
    status: "completed",
    title: "Request Submitted",
    description: "Service request created and submitted",
    timestamp: "Nov 18, 8:00 AM",
  },
  {
    status: "completed",
    title: "Approval Received",
    description: "Request approved by Manager - Jane Doe",
    timestamp: "Nov 18, 9:30 AM",
  },
  {
    status: "completed",
    title: "Resource Allocation",
    description: "Server resources provisioned",
    timestamp: "Nov 18, 10:45 AM",
  },
  {
    status: "active",
    title: "Environment Setup",
    description: "Installing and configuring API Manager",
    timestamp: "Nov 18, 11:00 AM",
  },
  { status: "pending", title: "Configuration", description: "Applying policies and settings" },
  { status: "pending", title: "Testing & Validation", description: "Verify environment is working correctly" },
  { status: "pending", title: "Handoff", description: "Environment ready for development team" },
];

export const MOCK_UPDATES = [
  {
    author: "You",
    timestamp: "3 days ago",
    content:
      "Requested database scaling for production environment. Current instance is at 85% capacity during peak hours.",
  },
  {
    author: "Infrastructure Team",
    timestamp: "2 days ago",
    content:
      "Request received and approved. We will schedule the scaling operation during the next maintenance window.",
  },
  {
    author: "Infrastructure Team",
    timestamp: "1 day ago",
    content:
      "Database instance has been successfully scaled to 2x capacity. Monitoring shows improved performance metrics.",
  },
  {
    author: "You",
    timestamp: "12 hours ago",
    content: "Confirmed - performance has improved significantly. Thank you!",
  },
];
