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

import {
  SUBSCRIPTION_STATUS,
  type SubscriptionStatus,
  SLA_STATUS,
  SUPPORT_TIER,
  PROJECT_TYPE,
  SYSTEM_HEALTH,
} from "@/constants/projectDetailsConstants";

/**
 * Formats a date string into "MMM DD, YYYY" format.
 * Example: "Jan 15, 2024"
 *
 * @param {string} dateString - The date string to format.
 * @returns {string} The formatted date string.
 */
export const formatProjectDate = (dateString: string): string => {
  if (!dateString) {
    return "";
  }
  const date = new Date(dateString);
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
};

/**
 * Determines the color of the SLA status chip based on the status string.
 *
 * @param {string} status - The SLA status string (e.g., "Good", "Bad", "Met", "Breached").
 * @returns {"success" | "error" | "default" | "warning"} The color for the Chip component.
 */
export const getSLAStatusColor = (
  status: string,
): "success" | "error" | "default" | "warning" => {
  const normalizedStatus = status?.toLowerCase();

  if (
    normalizedStatus === SLA_STATUS.GOOD.toLowerCase() ||
    normalizedStatus === "good"
  ) {
    return "success";
  }

  if (normalizedStatus === SLA_STATUS.BAD.toLowerCase()) {
    return "error";
  }

  return "default";
};

/**
 * Determines the color of the Support Tier chip based on the tier string.
 *
 * @param {string} tier - The support tier string (e.g., "Enterprise", "Standard").
 * @returns {"primary" | "info" | "default" | "success" | "warning" | "error"} The color for the Chip component.
 */
export const getSupportTierColor = (
  tier: string,
): "primary" | "info" | "default" | "success" | "warning" | "error" => {
  const normalizedTier = tier?.toLowerCase();

  if (normalizedTier === SUPPORT_TIER.ENTERPRISE.toLowerCase()) {
    return "warning";
  }

  if (normalizedTier === SUPPORT_TIER.STANDARD.toLowerCase()) {
    return "info";
  }

  return "default";
};

/**
 * Determines the color of the Project Type chip based on the type string.
 *
 * @param {string} type - The project type string (e.g., "Free", "Subscription").
 * @returns {"primary" | "info" | "default" | "success" | "warning" | "error"} The color for the Chip component.
 */
export const getProjectTypeColor = (
  type: string,
): "primary" | "info" | "default" | "success" | "warning" | "error" => {
  const normalizedType = type?.toLowerCase();

  if (normalizedType === PROJECT_TYPE.SUBSCRIPTION.toLowerCase()) {
    return "info";
  }

  if (normalizedType === PROJECT_TYPE.FREE.toLowerCase()) {
    return "warning";
  }

  return "default";
};

/**
 * Determines the color of the System Health chip based on the status string.
 *
 * @param {string} status - The system health status string (e.g., "Critical", "Healthy").
 * @returns {"primary" | "info" | "default" | "success" | "warning" | "error"} The color for the Chip component.
 */
export const getSystemHealthColor = (
  status: string,
): "primary" | "info" | "default" | "success" | "warning" | "error" => {
  const normalizedStatus = status?.toLowerCase();

  if (normalizedStatus === SYSTEM_HEALTH.HEALTHY.toLowerCase()) {
    return "success";
  }

  if (normalizedStatus === SYSTEM_HEALTH.CRITICAL.toLowerCase()) {
    return "error";
  }

  return "default";
};

/**
 * Determines the subscription status based on the end date.
 *
 * @param {string} endDateString - The subscription end date string.
 * @returns {SubscriptionStatus} The status string.
 */
export const getSubscriptionStatus = (
  endDateString: string,
): SubscriptionStatus => {
  if (!endDateString) {
    return SUBSCRIPTION_STATUS.ACTIVE;
  }

  const today = new Date();
  const endDate = new Date(endDateString);

  // Calculate difference in milliseconds
  const diffTime = endDate.getTime() - today.getTime();
  // Convert to days
  const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

  if (diffDays < 0) {
    return SUBSCRIPTION_STATUS.EXPIRED;
  }

  if (diffDays <= 30) {
    return SUBSCRIPTION_STATUS.EXPIRING_SOON;
  }

  return SUBSCRIPTION_STATUS.ACTIVE;
};

/**
 * Determines the color of the Subscription Status chip based on the status string.
 *
 * @param {SubscriptionStatus | string} status - The subscription status string.
 * @returns {"primary" | "info" | "default" | "success" | "warning" | "error"} The color for the Chip component.
 */
export const getSubscriptionColor = (
  status: SubscriptionStatus | string,
): "primary" | "info" | "default" | "success" | "warning" | "error" => {
  const normalizedStatus = status?.toLowerCase();

  if (normalizedStatus === SUBSCRIPTION_STATUS.EXPIRED.toLowerCase()) {
    return "error";
  }

  if (normalizedStatus === SUBSCRIPTION_STATUS.EXPIRING_SOON.toLowerCase()) {
    return "warning";
  }

  if (normalizedStatus === SUBSCRIPTION_STATUS.ACTIVE.toLowerCase()) {
    return "success";
  }

  return "default";
};

/**
 * Calculates the percentage of time elapsed between two dates.
 *
 * @param {string} start - The start date string.
 * @param {string} end - The end date string.
 * @returns {number} The progress percentage (0-100).
 */
export const calculateProgress = (start: string, end: string): number => {
  if (!start || !end) {
    return 0;
  }

  const startDate = new Date(start);
  const endDate = new Date(end);

  if (isNaN(startDate.getTime()) || isNaN(endDate.getTime())) {
    return 0;
  }

  const today = new Date();
  const total = endDate.getTime() - startDate.getTime();
  const elapsed = today.getTime() - startDate.getTime();

  if (total <= 0) return 100;
  return Math.min(100, Math.max(0, (elapsed / total) * 100));
};
