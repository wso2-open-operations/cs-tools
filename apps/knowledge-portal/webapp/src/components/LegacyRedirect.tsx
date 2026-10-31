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

import type { JSX } from "react";
import { Navigate, useLocation, useParams } from "react-router";
import { legacyKbTarget, previousArticleTarget, previousBrowseTarget } from "@utils/legacyUrl";

// Client-side redirects for URLs that earlier versions of the knowledge site
// published. Anything that cannot be mapped lands on the home page.

export function LegacyKbRedirect(): JSX.Element {
  const { search } = useLocation();
  return <Navigate to={legacyKbTarget(search) ?? "/"} replace />;
}

export function PreviousBrowseRedirect(): JSX.Element {
  const { search } = useLocation();
  return <Navigate to={previousBrowseTarget(search)} replace />;
}

export function PreviousArticleRedirect(): JSX.Element {
  const { id } = useParams();
  return <Navigate to={previousArticleTarget(id) ?? "/"} replace />;
}
