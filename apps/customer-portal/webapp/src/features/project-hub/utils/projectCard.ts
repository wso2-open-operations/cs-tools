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

/**
 * Formats a date string into "Created DD MMM YYYY" format.
 * Example: "2026-01-17 09:06:14" -> "Created 17 Jan 2026"
 *
 * @param {string} dateString - The date string to format.
 * @returns {string} The formatted date string.
 */
export const formatProjectDate = (dateString: string): string => {
  if (!dateString) return "";

  try {
    const date = new Date(dateString.replace(" ", "T"));

    // Check if date is valid
    if (isNaN(date.getTime())) {
      return dateString;
    }

    const day = date.getDate();
    const month = date.toLocaleString("en-US", { month: "short" });
    const year = date.getFullYear();

    return `Created ${day} ${month} ${year}`;
  } catch (error) {
    return dateString;
  }
};

/**
 * Returns the theme color for a given project status.
 *
 * @param {string} status - The project status.
 * @returns {"success" | "warning" | "error" | "default"} The theme color.
 */
export const getStatusColor = (
  status: string,
): "success" | "warning" | "error" | "default" => {
  const lowerStatus = status?.toLowerCase();
  switch (lowerStatus) {
    case "all good":
    case "good":
    case "healthy":
      return "success";
    case "need attention":
      return "warning";
    case "critical issues":
    case "bad":
    case "critical":
      return "error";
    default:
      return "default";
  }
};

/**
 * Removes HTML tags from a string.
 *
 * @param {string} str - The string to clean.
 * @returns {string} The cleaned string.
 */
export const stripHtmlTags = (str: string): string => {
  if (!str) return "";
  return str.replace(/<[^>]*>?/gm, "");
};
