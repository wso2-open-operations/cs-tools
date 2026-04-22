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

import { type JSX } from "react";
import ErrorPage from "./ErrorPage";
import illustration from "@assets/error/error-401.svg";

interface Error401PageProps {
  message?: string;
}

/**
 * Formats unauthorized response copy into a compact two-line message.
 *
 * @param {string | undefined} message - Raw API message.
 * @returns {string | undefined} Formatted message.
 */
function formatUnauthorizedMessage(message?: string): string | undefined {
  if (!message) {
    return undefined;
  }

  const normalized = message.replace(/\s+/g, " ").trim();
  const splitMarker = "Please try again.";

  if (normalized.includes(splitMarker)) {
    const [firstPart, secondPart] = normalized.split(splitMarker, 2);
    return `${firstPart.trim()} ${splitMarker}\n${(secondPart ?? "").trim()}`.trim();
  }

  const firstSentenceEnd = normalized.indexOf(".");
  if (firstSentenceEnd > -1 && firstSentenceEnd < normalized.length - 1) {
    const firstLine = normalized.slice(0, firstSentenceEnd + 1).trim();
    const secondLine = normalized.slice(firstSentenceEnd + 1).trim();
    return `${firstLine}\n${secondLine}`.trim();
  }

  return normalized;
}

export default function Error401Page({ message }: Error401PageProps): JSX.Element {
  return (
    <ErrorPage
      illustration={illustration}
      illustrationAlt="401 unauthorized illustration"
      description={
        formatUnauthorizedMessage(message) ??
        "You need to sign in to view this page.\nPlease authenticate and try again."
      }
    />
  );
}
