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

import type { NotificationFilter } from "@pages/NotificationsPage";
import type { NotificationsListItemProps } from "@components/features/notifications";

export const MOCK_NOTIFICATIONS: Record<NotificationFilter, NotificationsListItemProps[]> = {
  unread: [
    {
      type: "case",
      id: "CASE-2845",
      title: "Case CASE-2845 updated",
      description: "Sarah Chen added a comment: 'We identified the root cause of the timeout issues...'",
      timestamp: "2 mins ago",
      unread: true,
    },
    {
      type: "service",
      id: "SR-109",
      title: "Service degradation detected",
      description: "Monitoring reported elevated error rates on the Payment API.",
      timestamp: "15 mins ago",
      unread: true,
    },
  ],

  case: [
    {
      type: "case",
      id: "CASE-2819",
      title: "Case CASE-2819 assigned to you",
      description: "Alex Morgan assigned you as the owner of this case.",
      timestamp: "1 hour ago",
      unread: false,
    },
    {
      type: "case",
      id: "CASE-2793",
      title: "Case CASE-2793 resolved",
      description: "The issue was resolved and the case has been closed.",
      timestamp: "Yesterday",
      unread: false,
    },
  ],

  service: [
    {
      type: "service",
      id: "SR-102",
      title: "Service restored",
      description: "Email Delivery Service is now operating normally.",
      timestamp: "3 hours ago",
      unread: false,
    },
    {
      type: "service",
      id: "SR-097",
      title: "Scheduled maintenance",
      description: "Planned maintenance for the User Profile Service.",
      timestamp: "2 days ago",
      unread: false,
    },
  ],

  change: [
    {
      type: "change",
      id: "CR-445",
      title: "Change deployed",
      description: "Version 2.4.1 was successfully deployed to production.",
      timestamp: "4 hours ago",
      unread: false,
    },
    {
      type: "change",
      id: "CR-438",
      title: "Change approved",
      description: "Database index optimization change was approved.",
      timestamp: "2 days ago",
      unread: false,
    },
  ],
};
