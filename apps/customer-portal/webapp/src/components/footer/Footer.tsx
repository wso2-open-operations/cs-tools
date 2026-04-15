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
  COMPANY_NAME,
  PRIVACY_POLICY_URL,
  TERMS_OF_SERVICE_URL,
} from "@features/project-hub/constants/appLayoutConstants";
import { Footer as FooterUI } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Footer component.
 *
 * @returns {JSX.Element} The Footer component.
 */
export default function Footer(): JSX.Element {
  return (
    <FooterUI>
      <FooterUI.Copyright>
        © {new Date().getFullYear()} {COMPANY_NAME}. All rights reserved.
      </FooterUI.Copyright>
      <FooterUI.Link href={TERMS_OF_SERVICE_URL}>
        Terms & Conditions
      </FooterUI.Link>
      <FooterUI.Link href={PRIVACY_POLICY_URL}>Privacy Policy</FooterUI.Link>
    </FooterUI>
  );
}
