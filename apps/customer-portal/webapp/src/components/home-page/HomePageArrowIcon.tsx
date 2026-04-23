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

export interface HomePageArrowIconProps {
  size?: number;
}

/**
 * HomePageArrowIcon renders the right arrow icon used on HomePage CTAs.
 *
 * @param {HomePageArrowIconProps} props - Optional size in pixels.
 * @returns {JSX.Element} The rendered arrow SVG.
 */
export default function HomePageArrowIcon({
  size = 20,
}: HomePageArrowIconProps): JSX.Element {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      version="1.1"
      viewBox="0 0 22.1 22.5"
      aria-hidden="true"
      width={size}
      height={size}
      style={{
        display: "block",
        flexShrink: 0,
        transition: "transform 0.3s ease",
      }}
    >
      <path
        d="M21.7,10.2L12,.4c-.6-.6-1.6-.6-2.2,0-.6.6-.6,1.6,0,2.2l7.1,7.1H0v3.1h16.9l-7.1,7.1c-.6.6-.6,1.6,0,2.2.6.6,1.6.6,2.2,0l9.7-9.7c.6-.6.6-1.6,0-2.2Z"
        fill="currentColor"
      />
    </svg>
  );
}

