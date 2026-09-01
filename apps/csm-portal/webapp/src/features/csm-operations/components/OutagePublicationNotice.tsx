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

import { Alert, Checkbox, FormControlLabel, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";

interface OutagePublicationNoticeProps {
  /** Whether a configuration item is currently selected on this outage. */
  hasConfigurationItem: boolean;
  /** Live monitored-cloud list from `GET /outages/metadata`
   * (`statusPageClouds`) — informational only; whether THIS outage's chosen
   * configuration item resolves to one of them is computed server-side
   * (`OutageUtils.resolvePublication`, which walks the CI's class ancestry),
   * so the frontend cannot predict it exactly. Showing the checkbox
   * whenever a configuration item is set, with this list as context, warns
   * proactively rather than only after a `409`. */
  monitoredClouds: string[] | undefined;
  acknowledged: boolean;
  onAcknowledgedChange: (next: boolean) => void;
  disabled?: boolean;
}

/**
 * Publication-safety notice shown whenever an outage carries (or is about
 * to carry) a configuration item. `configurationItemId` is "the publish
 * switch" (see `CHANGES-outage-api.md`'s "Publication safety" section): if
 * it resolves to a monitored cloud, this outage's communications — and an
 * `external` channel entry in particular — become visible on the public
 * status page. `acknowledgePublicPublication` is required by the backend
 * whenever that resolution is true; asking for it up front here, rather
 * than waiting for the `409`, is the whole point of this component.
 */
export default function OutagePublicationNotice({
  hasConfigurationItem,
  monitoredClouds,
  acknowledged,
  onAcknowledgedChange,
  disabled,
}: OutagePublicationNoticeProps): JSX.Element | null {
  if (!hasConfigurationItem) return null;

  return (
    <Alert severity="warning" sx={{ alignItems: "flex-start" }}>
      <Typography variant="body2" sx={{ mb: 0.5 }}>
        This outage links a configuration item. If it belongs to a monitored
        cloud
        {monitoredClouds && monitoredClouds.length > 0
          ? ` (${monitoredClouds.join(", ")})`
          : ""}
        , its communications — including any external update you post — will
        be visible on the public status page.
      </Typography>
      <FormControlLabel
        control={
          <Checkbox
            checked={acknowledged}
            onChange={(e) => onAcknowledgedChange(e.target.checked)}
            disabled={disabled}
          />
        }
        label="I understand this outage may be publicly visible."
      />
    </Alert>
  );
}
