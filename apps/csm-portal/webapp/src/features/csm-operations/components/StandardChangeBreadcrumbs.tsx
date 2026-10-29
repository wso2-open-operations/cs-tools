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

import { Breadcrumbs, Link, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { Link as RouterLink } from "react-router";
import {
  CHANGE_REQUESTS_PATH,
  changeRequestsViewPath,
  STANDARD_CHANGES_VIEW_LABELS,
  type StandardChangesView,
} from "@features/csm-operations/utils/standardChanges";

/**
 * "Change requests › Standard change catalogue › STD0000001" above a template's
 * or a proposal's own page: both are part of Operations > Change requests, and
 * the trail says so and links back to the list and to the sub-view. It is a
 * fixed trail beside the page's own Back (which returns to wherever the page
 * was opened from).
 */
export default function StandardChangeBreadcrumbs({
  view,
  current,
}: {
  view: StandardChangesView;
  /** The record's number, once loaded. */
  current?: string;
}): JSX.Element {
  return (
    <Breadcrumbs aria-label="Breadcrumb" separator="›" sx={{ typography: "body2" }}>
      <Link component={RouterLink} to={CHANGE_REQUESTS_PATH} underline="hover" color="inherit">
        Change requests
      </Link>
      <Link component={RouterLink} to={changeRequestsViewPath(view)} underline="hover" color="inherit">
        {STANDARD_CHANGES_VIEW_LABELS[view]}
      </Link>
      {current && (
        <Typography variant="body2" color="text.primary" aria-current="page">
          {current}
        </Typography>
      )}
    </Breadcrumbs>
  );
}
