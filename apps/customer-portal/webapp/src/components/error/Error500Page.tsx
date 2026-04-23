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
import illustration from "@assets/error/error-500.svg";

interface Error500PageProps {
  message?: string;
}

/**
 * Full-area state for server errors (5xx) and unknown failures, matching
 * the layout scale of {@link Error404Page} and {@link Error400Page}.
 *
 * @param {Error500PageProps} props - Optional message from the server or fallback copy.
 * @returns {JSX.Element} Server error illustration and copy.
 */
export default function Error500Page({ message }: Error500PageProps): JSX.Element {
  return (
    <ErrorPage
      illustration={illustration}
      illustrationAlt="500 server error illustration"
      description={
        message?.trim() ||
        "Something went wrong on our side. Please try again in a few moments."
      }
    />
  );
}
